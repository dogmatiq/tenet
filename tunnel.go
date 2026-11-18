package tenet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
)

// subprotocol is the WebSocket subprotocol that names the version of the wire
// format that a tunnel uses. [Connect] offers it, and [Accept] refuses a
// request that doesn't offer it.
const subprotocol = "tenet.v1"

// offersSubprotocol returns true if r offers [subprotocol] in its
// Sec-WebSocket-Protocol header.
//
// The header holds a comma-separated list, and may appear more than once.
func offersSubprotocol(r *http.Request) bool {
	for _, v := range r.Header.Values("Sec-WebSocket-Protocol") {
		for p := range strings.SplitSeq(v, ",") {
			if strings.TrimSpace(p) == subprotocol {
				return true
			}
		}
	}
	return false
}

// A tunnel is one WebSocket between a client and a server, and the yamux
// session that multiplexes connections over it.
type tunnel struct {
	session *yamux.Session

	// peer is the role of the other end, "client" or "server", for use in
	// errors.
	peer string

	// handle handles each stream that the other end opens. If it is nil, the
	// other end may not open streams.
	handle func(*tunnel, *yamux.Stream)

	// ended is closed when the tunnel has ended, after every stream on the
	// session has closed.
	ended chan struct{}

	m     sync.Mutex
	cause error

	// conns is the number of connections that this end has made from the
	// streams that the other end opened, and that are not closed yet.
	conns int

	// idleCause is the cause that endWhenIdle was given, or nil if it hasn't
	// been called.
	idleCause error
}

// newTunnel starts a tunnel over ws. It calls handle in a new goroutine for
// each stream that the other end opens. If handle is nil, the tunnel ends as
// soon as the other end opens a stream.
//
// newSession is [yamux.Client] or [yamux.Server]. The two differ only in the
// IDs that they give the streams that they open, so the two ends of a tunnel
// must use different ones.
func newTunnel(
	ws *websocket.Conn,
	cfg tunnelConfig,
	newSession func(io.ReadWriteCloser, *yamux.Config) (*yamux.Session, error),
	peer string,
	handle func(*tunnel, *yamux.Stream),
) (*tunnel, error) {
	// The context bounds the lifetime of the net.Conn, which ends only when the
	// session closes it.
	conn := websocket.NetConn(context.Background(), ws, websocket.MessageBinary)

	session, err := newSession(conn, cfg.yamuxConfig())
	if err != nil {
		ws.CloseNow()
		return nil, err
	}

	t := &tunnel{
		session: session,
		peer:    peer,
		handle:  handle,
		ended:   make(chan struct{}),
	}

	go t.run()

	return t, nil
}

// yamuxConfig returns the configuration of the yamux session for a tunnel.
func (c tunnelConfig) yamuxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.KeepAliveInterval = c.keepAliveInterval
	cfg.ConnectionWriteTimeout = c.keepAliveTimeout
	cfg.LogOutput = io.Discard // yamux writes to standard error by default

	// By default, yamux closes the whole session when the other end doesn't
	// acknowledge a new stream within 75 seconds. A tunnel adds no timeout to
	// dialing.
	cfg.StreamOpenTimeout = 0

	return cfg
}

// Done returns a channel that is closed when the tunnel has ended.
func (t *tunnel) Done() <-chan struct{} {
	return t.ended
}

// Err returns nil until the channel that [tunnel.Done] returns is closed, and
// then why the tunnel ended.
func (t *tunnel) Err() error {
	select {
	case <-t.ended:
		t.m.Lock()
		defer t.m.Unlock()
		return t.cause
	default:
		return nil
	}
}

// close ends the tunnel, and waits until it has ended. [tunnel.Err] returns
// cause, unless the tunnel had already ended for another reason.
func (t *tunnel) close(cause error) {
	t.setCause(cause)
	t.session.Close()
	<-t.ended
}

// connOpened records that this end has made a connection from a stream that
// the other end opened. connClosed must be called when it closes.
func (t *tunnel) connOpened() {
	t.m.Lock()
	defer t.m.Unlock()
	t.conns++
}

// connClosed records that a connection that connOpened recorded has closed.
// If endWhenIdle has been called and this was the last connection, the tunnel
// ends.
func (t *tunnel) connClosed() {
	t.m.Lock()
	t.conns--
	cause := t.idleCause
	idle := cause != nil && t.conns == 0
	t.m.Unlock()

	if idle {
		// close waits for the WebSocket's close handshake, which the
		// connection's Close shouldn't wait for.
		go t.close(cause)
	}
}

// endWhenIdle ends the tunnel because of cause as soon as no connection that
// connOpened recorded is open, and waits until it has ended.
func (t *tunnel) endWhenIdle(cause error) {
	t.m.Lock()
	t.idleCause = cause
	idle := t.conns == 0
	t.m.Unlock()

	if idle {
		t.close(cause)
	}

	<-t.ended
}

// open opens a new stream to the other end. It returns an error that wraps
// ctx.Err() as soon as ctx ends, or the tunnel's error once it has ended.
func (t *tunnel) open(ctx context.Context) (*yamux.Stream, error) {
	type result struct {
		stream *yamux.Stream
		err    error
	}

	// OpenStream takes no context, and blocks while too many streams wait for
	// the other end to acknowledge them.
	done := make(chan result, 1)
	go func() {
		s, err := t.session.OpenStream()
		done <- result{s, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			return nil, t.failure(ctx, r.err)
		}
		return r.stream, nil

	case <-ctx.Done():
		go func() {
			if r := <-done; r.stream != nil {
				r.stream.Close()
			}
		}()
		return nil, ctx.Err()
	}
}

// failure returns the error to report when an operation on the session fails
// with err. Once the session has ended, it waits until the tunnel has ended,
// and returns the tunnel's error.
func (t *tunnel) failure(ctx context.Context, err error) error {
	t.checkWrite(err)

	if !t.session.IsClosed() {
		return err
	}

	select {
	case <-t.ended:
		return t.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run accepts the streams that the other end opens until the session ends,
// and then ends the tunnel.
func (t *tunnel) run() {
	for {
		stream, err := t.session.AcceptStream()

		if err == nil {
			if t.handle != nil {
				go t.handle(t, stream)
				continue
			}

			// yamux would keep whatever the other end writes to an unwanted
			// stream, even after this end closes it, so the tunnel ends
			// instead. AcceptStream then returns the session's error, and the
			// loop ends below.
			t.setCause(fmt.Errorf("the %s broke the tunneling protocol by opening a stream", t.peer))
			t.session.Close()
			continue
		}

		// After the session ends, AcceptStream returns why it ended. Before,
		// it returns an error only when it fails to answer one stream.
		t.checkWrite(err)
		if t.session.IsClosed() {
			t.end(err)
			return
		}
	}
}

// checkWrite ends the tunnel if err shows that the other end didn't accept a
// write within the keep-alive timeout.
//
// yamux returns that error to the writer, but doesn't end the session, so each
// operation on the session that can write passes its error here.
func (t *tunnel) checkWrite(err error) {
	if !errors.Is(err, yamux.ErrConnectionWriteTimeout) {
		return
	}

	t.setCause(fmt.Errorf("the %s didn't accept a write within the keep-alive timeout: %w", t.peer, err))
	t.session.Close()
}

// end ends the tunnel after its session has ended because of err.
func (t *tunnel) end(err error) {
	if err == io.EOF {
		err = fmt.Errorf("the %s closed the tunnel: %w", t.peer, err)
	} else {
		err = fmt.Errorf("the tunnel failed: %w", err)
	}
	t.setCause(err)

	// The session may still be closing in another goroutine. Close waits
	// until every stream has closed.
	t.session.Close()

	close(t.ended)
}

// setCause records why the tunnel ended, unless a reason is already recorded.
func (t *tunnel) setCause(err error) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.cause == nil {
		t.cause = err
	}
}

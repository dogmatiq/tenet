package tenet

import (
	"context"
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

	// ended is closed when the tunnel has ended, after every stream on the
	// session has closed.
	ended chan struct{}

	m     sync.Mutex
	cause error
}

// newTunnel starts a tunnel over ws.
//
// newSession is [yamux.Client] or [yamux.Server]. The two differ only in the
// IDs that they give the streams that they open, so the two ends of a tunnel
// must use different ones.
func newTunnel(
	ws *websocket.Conn,
	cfg tunnelConfig,
	newSession func(io.ReadWriteCloser, *yamux.Config) (*yamux.Session, error),
	peer string,
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

// run accepts the streams that the other end opens until the session ends,
// and then ends the tunnel.
func (t *tunnel) run() {
	for {
		stream, err := t.session.AcceptStream()

		if err == nil {
			// This end doesn't expect streams, so it closes each one that the
			// other end opens.
			stream.Close()
			continue
		}

		// After the session ends, AcceptStream returns why it ended. Before,
		// it returns an error only when it fails to answer one stream.
		if t.session.IsClosed() {
			t.end(err)
			return
		}
	}
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

package tenet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
)

// Accept upgrades r to a WebSocket, and returns a [Dialer] that opens
// connections back to the [Client] that sent r, through the WebSocket.
//
// It is intended to be used within an [http.Handler] that is serving a request
// initiated by [Connect].
//
// If the upgrade fails, for example because the [Client] that sent r uses a
// different version of the tunneling wire format, it writes an HTTP error
// response to w, and returns an error.
//
// The returned [Dialer] is not tied to the lifetime of the HTTP handler. It may
// be used until the tunnel ends.
func Accept(
	w http.ResponseWriter,
	r *http.Request,
	options ...AcceptOption,
) (*Dialer, error) {
	cfg := acceptConfig{
		tunnelConfig: newTunnelConfig(),
	}
	for _, o := range options {
		o.applyToAccept(&cfg)
	}

	// The WebSocket library negotiates no subprotocol for a client that offers
	// none, and the error response can't be written after the upgrade, so
	// Accept checks the subprotocol itself, first.
	if !offersSubprotocol(r) {
		msg := fmt.Sprintf("the client must offer the %q WebSocket subprotocol", subprotocol)
		http.Error(w, msg, http.StatusBadRequest)
		return nil, errors.New(msg)
	}

	ws, err := websocket.Accept(
		w,
		r,
		&websocket.AcceptOptions{
			Subprotocols: []string{subprotocol},
		},
	)
	if err != nil {
		// websocket.Accept has already written an HTTP error response.
		return nil, err
	}

	// The Dialer opens the streams, so its session is yamux's client. The
	// client may not open any.
	t, err := newTunnel(ws, cfg.tunnelConfig, yamux.Client, "client", nil)
	if err != nil {
		return nil, err
	}

	return &Dialer{t}, nil
}

// A Dialer is the server's half of a tunnel to a [Client].
//
// Obtain a Dialer by calling [Accept] within an HTTP handler that is serving a
// request initiated by [Connect].
type Dialer struct {
	tunnel *tunnel
}

// DialContext opens a connection to the client's [Listener] over the tunnel.
//
// The network and address are used to route the connection to the listener with
// the same network and address, as created by [Client.Listen]. If the client
// has no such [Listener], DialContext returns an error that wraps
// [syscall.ECONNREFUSED]. It returns an error that wraps [syscall.ECONNRESET]
// if that [Listener] already has too many connections waiting to be accepted.
// It returns an error that wraps [syscall.EINVAL] if network is longer than 64
// bytes or address is longer than 448 bytes.
//
// The connection's [net.Conn.RemoteAddr] method returns an address matching
// the given network and address. Its [net.Conn.LocalAddr] method returns an
// address with the same network, whose string form is empty.
//
// DialContext has the same signature as [net.Dialer.DialContext], so it can be
// used wherever a dial function is needed, such as
// [http.Transport.DialContext].
func (d *Dialer) DialContext(
	ctx context.Context,
	network, address string,
) (net.Conn, error) {
	remote := addr{network, address}

	fail := func(err error) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Net: network, Addr: remote, Err: err}
	}

	if err := remote.validate(); err != nil {
		return fail(err)
	}

	s, err := d.tunnel.open(ctx)
	if err != nil {
		return fail(err)
	}

	// Interrupt the exchange below as soon as ctx ends.
	stop := context.AfterFunc(ctx, func() {
		s.SetDeadline(time.Unix(1, 0))
	})

	ans, err := exchange(s, remote)

	if !stop() {
		s.Close()
		return fail(ctx.Err())
	}

	if err != nil {
		s.Close()

		if d.tunnel.session.IsClosed() {
			return fail(d.tunnel.failure(ctx, err))
		}

		// The client closes or resets the stream without an answer only when
		// it can't take the stream at all, such as when it can't read the
		// address.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, yamux.ErrConnectionReset) {
			return fail(syscall.ECONNRESET)
		}

		return fail(err)
	}

	switch ans {
	case refused:
		s.Close()
		return fail(syscall.ECONNREFUSED)
	case backlogFull:
		s.Close()
		return fail(syscall.ECONNRESET)
	}

	return &conn{
		stream: s,
		local:  remote.unnamed(),
		remote: remote,
	}, nil
}

// Done returns a channel that is closed when the tunnel ends. By then, every
// connection through the tunnel has ended, and calls to [Dialer.DialContext]
// fail.
func (d *Dialer) Done() <-chan struct{} {
	return d.tunnel.Done()
}

// Err returns nil until the channel that [Dialer.Done] returns is closed. Then
// it returns why the tunnel ended. It returns [net.ErrClosed] if [Dialer.Close]
// ended it.
func (d *Dialer) Err() error {
	return d.tunnel.Err()
}

// Close ends the tunnel and every connection through it.
//
// Close may be called more than once. If the tunnel has already ended, Close
// does nothing. Close always returns nil. It returns an error only so that a
// [Dialer] is an [io.Closer].
func (d *Dialer) Close() error {
	d.tunnel.close(net.ErrClosed)
	return nil
}

package tenet

import (
	"net"
)

// A Listener accepts the connections that a [Dialer] dials through the
// WebSocket that [Listen] connects.
//
// A Listener is a [net.Listener], so a server can serve it in the same way as
// any other [net.Listener].
type Listener struct{}

// Accept waits for the next connection that the [Dialer] at the other end of
// the WebSocket dials, and returns it. It keeps waiting while the [Listener]
// connects again after a WebSocket ends.
//
// It returns [net.ErrClosed] once [Listener.Close] is called, and an error that
// wraps ctx.Err() once the context given to [Listen] is canceled. Those are the
// only errors that it returns, so the server that serves the [Listener] keeps
// serving it while the [Listener] connects again.
//
// The [net.Conn.LocalAddr] and [net.Conn.RemoteAddr] methods of the connection
// return addresses whose [net.Addr.Network] method returns "tenet". Their
// [net.Addr.String] methods describe the WebSocket server, for display only.
func (l *Listener) Accept() (net.Conn, error) {
	panic("not implemented")
}

// Close stops the [Listener] from connecting again. No new connections open,
// and new calls to [Dialer.Dial] at the other end of the WebSocket fail at
// once. The connections that are already open stay open until the server that
// serves the [Listener] closes them, and the WebSocket ends after the last one
// closes.
//
// Close may be called more than once, and always returns nil.
func (l *Listener) Close() error {
	panic("not implemented")
}

// Addr returns the [Listener]'s address. Its [net.Addr.Network] method returns
// "tenet". Its [net.Addr.String] method describes the WebSocket server, for
// display only.
func (l *Listener) Addr() net.Addr {
	panic("not implemented")
}

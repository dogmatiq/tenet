package tenet

import (
	"net"
)

// A Listener is a [net.Listener] for the connections that the server opens with
// [Dialer.DialContext] for one network and address.
//
// Obtain a Listener by calling [Client.Listen]. A Listener may be used until
// [Listener.Close] or [Client.Close] is called.
type Listener struct{}

// Accept waits for the next connection that the server opens to the
// [Listener]'s network and address, and returns the [Listener]'s end of it.
//
// It returns [net.ErrClosed] once [Listener.Close] or [Client.Close] is called.
// It returns no other error, so a function such as [http.Server.Serve] that
// serves the [Listener] keeps serving it while the [Client] connects again.
//
// The connection's [net.Conn.LocalAddr] method returns the address that
// [Listener.Addr] returns. Its [net.Conn.RemoteAddr] method returns an address
// with the same network, whose string form is empty.
func (l *Listener) Accept() (net.Conn, error) {
	panic("not implemented")
}

// Close stops the [Listener] from accepting connections. Connections that the
// server has opened, but that [Listener.Accept] has not returned, are closed.
// Connections that [Listener.Accept] has returned stay open until they are
// closed.
//
// After Close, a call to [Dialer.DialContext] for the [Listener]'s network and
// address returns an error that wraps [syscall.ECONNREFUSED], and
// [Client.Listen] may be called again for that network and address.
//
// Close may be called more than once, and always returns nil.
func (l *Listener) Close() error {
	panic("not implemented")
}

// Addr returns the [Listener]'s address. Its [net.Addr.Network] and
// [net.Addr.String] methods return the network and address that were given to
// [Client.Listen].
func (l *Listener) Addr() net.Addr {
	panic("not implemented")
}

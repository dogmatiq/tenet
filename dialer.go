package tenet

import (
	"context"
	"net"
	"net/http"
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
func Accept(w http.ResponseWriter, r *http.Request) (*Dialer, error) {
	panic("not implemented")
}

// A Dialer is the server's half of a tunnel to a [Client].
//
// Obtain a Dialer by calling [Accept] within an HTTP handler that is serving a
// request initiated by [Connect].
type Dialer struct{}

// DialContext opens a connection to the client's [Listener] over the tunnel.
//
// The network and address are used to route the connection to the listener with
// the same network and address, as created by [Client.Listen]. If the client
// has no such [Listener], DialContext returns an error that wraps
// [syscall.ECONNREFUSED].
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
	panic("not implemented")
}

// Done returns a channel that is closed when the tunnel ends. By then, every
// connection through the tunnel has ended, and calls to [Dialer.DialContext]
// fail.
func (d *Dialer) Done() <-chan struct{} {
	panic("not implemented")
}

// Err returns nil until the channel that [Dialer.Done] returns is closed. Then
// it returns why the tunnel ended. It returns [net.ErrClosed] if [Dialer.Close]
// ended it.
func (d *Dialer) Err() error {
	panic("not implemented")
}

// Close ends the tunnel and every connection through it.
//
// Close may be called more than once. If the tunnel has already ended, Close
// does nothing. Close always returns nil. It returns an error only so that a
// [Dialer] is an [io.Closer].
func (d *Dialer) Close() error {
	panic("not implemented")
}

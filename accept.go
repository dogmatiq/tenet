package tenet

import (
	"context"
	"net"
	"net/http"
)

// Accept upgrades r to a WebSocket, and returns a [Dialer] that dials
// connections through it to the [Listener] at its other end.
//
// Call it from an [http.Handler]. If the upgrade fails, Accept writes an HTTP
// error response to w, and returns an error.
//
// The [Dialer] keeps working after the handler returns.
func Accept(w http.ResponseWriter, r *http.Request) (*Dialer, error) {
	panic("not implemented")
}

// A Dialer dials connections through one WebSocket to the [Listener] at its
// other end.
//
// Each call to [Accept] returns a new Dialer, for a new WebSocket.
type Dialer struct{}

// Dial opens a new connection to the [Listener] at the other end of the
// WebSocket.
//
// If ctx ends first, Dial returns at once with an error that wraps ctx.Err(),
// and leaves nothing waiting in the tunnel. Dial fails at once if the
// [Listener] is closed, and with an error that wraps [Dialer.Err] once the
// WebSocket has ended.
//
// The [net.Conn.LocalAddr] and [net.Conn.RemoteAddr] methods of the connection
// return addresses whose [net.Addr.Network] method returns "tenet". Their
// [net.Addr.String] methods describe the other end of the WebSocket, for
// display only.
func (d *Dialer) Dial(ctx context.Context) (net.Conn, error) {
	panic("not implemented")
}

// Done returns a channel that is closed when the WebSocket ends.
func (d *Dialer) Done() <-chan struct{} {
	panic("not implemented")
}

// Err returns nil until the channel that [Dialer.Done] returns is closed. Then
// it returns why the WebSocket ended. It returns [net.ErrClosed] if
// [Dialer.Close] ended it.
func (d *Dialer) Err() error {
	panic("not implemented")
}

// Close ends the WebSocket and every connection on it. It may be called more
// than once. If the WebSocket has already ended, Close does nothing.
//
// Close always returns nil. It returns an error only so that a [Dialer] is an
// [io.Closer].
func (d *Dialer) Close() error {
	panic("not implemented")
}

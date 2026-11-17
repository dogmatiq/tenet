package tenet

import (
	"context"
	"net"
	"net/http"
)

// Listen connects to the WebSocket server at url, and returns a [Listener] that
// accepts the connections that the WebSocket server dials through the
// WebSocket. The WebSocket server's [http.Handler] passes the request to
// [Accept], and dials with the [Dialer] that [Accept] returns.
//
// Listen returns at once, and connects in the background. It connects again
// whenever the WebSocket ends, until ctx is canceled or the [Listener] is
// closed. Canceling ctx ends the WebSocket and every connection on it at once.
//
// Each attempt to connect is an HTTP request made with httpClient, so its
// redirects, proxy and TLS settings apply to it as they do to any other
// request. Each attempt sends a copy of header. A value that changes between
// attempts, such as a token that expires, belongs in httpClient's
// [http.Client.Transport] instead.
//
// Listen returns an error only if url isn't a valid URL, or its scheme isn't
// "ws" or "wss".
func Listen(
	ctx context.Context,
	httpClient *http.Client,
	url string,
	header http.Header,
) (*Listener, error) {
	panic("not implemented")
}

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

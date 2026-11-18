package tenet

// Connect returns a [Client] that connects to the server at the given URL.
//
// The server must pass each request for url to [Accept]. Connect returns an
// error if url isn't a valid URL with the scheme "ws" or "wss".
//
// Connect returns immediately, and the [Client] connects in the background.
// Whenever the tunnel ends, or an attempt to connect fails, the [Client] waits,
// and then connects again.
//
// Each attempt to connect is an HTTP request, made with the [http.Client] that
// [WithHTTPClient] sets, or [http.DefaultClient] by default.
func Connect(url string, options ...ConnectOption) (*Client, error) {
	panic("not implemented")
}

// A Client keeps a tunnel open to a server, so that the server can open
// connections back to the client via the tunnel.
//
// Obtain a Client by calling [Connect]. A Client may be used until
// [Client.Close] is called.
type Client struct{}

// Listen returns a [Listener] for network and address. Each connection that
// the server opens with [Dialer.DialContext] for exactly the same network and
// address goes to that [Listener].
//
// network and address may be any strings, such as "tcp" and
// "api.internal:443". network is limited to 64 bytes, and address to 448 bytes.
//
// The [Listener] may be used until [Listener.Close] or [Client.Close] is
// called.
//
// Listen returns an error that wraps [syscall.EADDRINUSE] if the [Client]
// already has an open [Listener] for the same network and address, as a TCP
// listen on a port that is already in use does. It returns an error that wraps
// [syscall.EINVAL] if network or address is too long. It returns
// [net.ErrClosed] once [Client.Close] has been called.
func (c *Client) Listen(network, address string) (*Listener, error) {
	panic("not implemented")
}

// Close stops the [Client] from connecting again, and closes each of its
// [Listener] values. Connections that are already open stay open until they
// are closed, and the tunnel ends after the last one closes.
//
// Close may be called more than once, and always returns nil.
func (c *Client) Close() error {
	panic("not implemented")
}

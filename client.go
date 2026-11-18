package tenet

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
)

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
	u, err := parseURL(url)
	if err != nil {
		return nil, err
	}

	cfg := connectConfig{
		tunnelConfig: newTunnelConfig(),
		httpClient:   http.DefaultClient,
		observer:     nopObserver{},
	}
	for _, o := range options {
		o.applyToConnect(&cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		url:    u.String(),
		cfg:    cfg,
		closed: ctx,
		close:  cancel,
	}

	go c.run()

	return c, nil
}

// parseURL parses the URL that is given to [Connect].
func parseURL(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("the URL %q must have the scheme \"ws\" or \"wss\"", s)
	}

	return u, nil
}

// A Client keeps a tunnel open to a server, so that the server can open
// connections back to the client via the tunnel.
//
// Obtain a Client by calling [Connect]. A Client may be used until
// [Client.Close] is called.
type Client struct {
	url string
	cfg connectConfig

	// closed is canceled by close, which [Client.Close] calls. It also bounds
	// each attempt to connect.
	closed context.Context
	close  context.CancelFunc
}

// run connects to the server, and waits until the tunnel ends or the [Client]
// is closed.
func (c *Client) run() {
	t, err := c.open()
	if err != nil {
		return
	}

	c.cfg.observer.TunnelOpened()

	select {
	case <-t.Done():
	case <-c.closed.Done():
		t.close(net.ErrClosed)
	}

	c.cfg.observer.TunnelEnded(t.Err())
}

// open makes one attempt to open a tunnel to the server.
func (c *Client) open() (*tunnel, error) {
	ws, _, err := websocket.Dial(
		c.closed,
		c.url,
		&websocket.DialOptions{
			HTTPClient:   c.cfg.httpClient,
			HTTPHeader:   c.cfg.header,
			Subprotocols: []string{subprotocol},
		},
	)
	if err != nil {
		return nil, err
	}

	// A server that doesn't know the subprotocol may still accept the
	// WebSocket without choosing it.
	if ws.Subprotocol() != subprotocol {
		ws.CloseNow()
		return nil, fmt.Errorf("the server didn't choose the %q WebSocket subprotocol", subprotocol)
	}

	// The Dialer at the other end opens the streams, so this end's session is
	// yamux's server.
	return newTunnel(ws, c.cfg.tunnelConfig, yamux.Server, "server")
}

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
	c.close()
	return nil
}

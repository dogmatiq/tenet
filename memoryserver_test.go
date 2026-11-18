package tenet_test

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
)

// memoryServer is an HTTP server that clients reach through in-memory pipes
// instead of the network, so that a test can run in a [synctest] bubble, where
// time is fake.
type memoryServer struct {
	// URL is the URL to give to [Connect].
	URL string

	// Client is the HTTP client to give to [WithHTTPClient].
	Client *http.Client

	server    *http.Server
	transport *http.Transport
	conns     chan net.Conn
	closed    chan struct{}
	closeOnce sync.Once
}

// startMemoryServer starts a [memoryServer] that serves h, and stops it when
// the test ends.
func startMemoryServer(t testing.TB, h http.Handler) *memoryServer {
	t.Helper()

	s := &memoryServer{
		URL:    "ws://tunnel.test/",
		conns:  make(chan net.Conn),
		closed: make(chan struct{}),
		server: &http.Server{Handler: h},
	}

	s.transport = &http.Transport{DialContext: s.dial}
	s.Client = &http.Client{Transport: s.transport}

	go s.server.Serve(s)

	t.Cleanup(func() {
		s.server.Close()
		s.transport.CloseIdleConnections()
	})

	return s
}

// dial returns one end of a new pipe, and passes the other end to the server.
func (s *memoryServer) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()

	select {
	case s.conns <- server:
		return client, nil
	case <-s.closed:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Accept returns the server's end of the next pipe. It makes memoryServer a
// [net.Listener].
func (s *memoryServer) Accept() (net.Conn, error) {
	select {
	case c := <-s.conns:
		return c, nil
	case <-s.closed:
		return nil, net.ErrClosed
	}
}

// Close stops Accept.
func (s *memoryServer) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

// Addr returns a placeholder address.
func (s *memoryServer) Addr() net.Addr {
	return &net.UnixAddr{Name: "tunnel.test", Net: "memory"}
}

package tenet_test

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memoryServer is an HTTP server that clients reach through in-memory pipes
// instead of the network, so that a test can run in a [synctest] bubble, where
// time is fake. Its network can also be lost without a word, as a real network
// can.
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
	lost      atomic.Bool
	stalled   atomic.Bool

	m     sync.Mutex
	pipes []net.Conn
}

// LoseNetwork makes the network between the server and its clients go silent.
// After it, each write succeeds but never arrives, and closing one end of a
// connection doesn't reach the other end.
func (s *memoryServer) LoseNetwork() {
	s.lost.Store(true)
}

// StallNetwork makes the network between the server and its clients stop
// moving data, as a real network does when the other end stops reading. After
// it, each write blocks until its end is closed, and closing one end of a
// connection doesn't reach the other end.
func (s *memoryServer) StallNetwork() {
	s.stalled.Store(true)
	s.lost.Store(true)
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

		s.m.Lock()
		defer s.m.Unlock()
		for _, p := range s.pipes {
			p.Close()
		}
	})

	return s
}

// dial returns one end of a new pipe, and passes the other end to the server.
func (s *memoryServer) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()

	s.m.Lock()
	s.pipes = append(s.pipes, client, server)
	s.m.Unlock()

	select {
	case s.conns <- s.newConn(server):
		return s.newConn(client), nil
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

func (s *memoryServer) newConn(pipe net.Conn) *memoryConn {
	return &memoryConn{
		Conn:   pipe,
		server: s,
		closed: make(chan struct{}),
	}
}

// memoryConn is one end of a connection through a [memoryServer]'s network.
type memoryConn struct {
	net.Conn // one end of a pipe

	server    *memoryServer
	closed    chan struct{}
	closeOnce sync.Once
}

func (c *memoryConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)

	if c.server.lost.Load() {
		// Nothing arrives once the network is lost.
		<-c.closed
		return 0, net.ErrClosed
	}

	return n, err
}

func (c *memoryConn) Write(b []byte) (int, error) {
	if c.server.stalled.Load() {
		<-c.closed
		return 0, net.ErrClosed
	}
	if c.server.lost.Load() {
		return len(b), nil
	}
	return c.Conn.Write(b)
}

func (c *memoryConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })

	if !c.server.lost.Load() {
		return c.Conn.Close()
	}

	// Closing the pipe would reach the other end, so only unblock this end.
	past := time.Unix(1, 0)
	c.Conn.SetDeadline(past)

	return nil
}

package tenet_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/dogmatiq/tenet"
)

// testServer is an HTTP server whose handler passes each request to [Accept].
type testServer struct {
	// URL is the server's URL, with the scheme "ws" or "wss".
	URL string

	// Server is the underlying HTTP server.
	Server *httptest.Server

	// Headers receives the headers of each request.
	Headers chan http.Header

	// Dialers receives each [Dialer] that [Accept] returns.
	Dialers chan *Dialer

	// Errors receives each error that [Accept] returns.
	Errors chan error
}

// startServer starts a [testServer] that passes options to [Accept].
func startServer(t testing.TB, options ...AcceptOption) *testServer {
	t.Helper()
	return start(t, httptest.NewServer, "ws", options...)
}

// startTLSServer starts a [testServer] that serves TLS, and passes options to
// [Accept].
func startTLSServer(t testing.TB, options ...AcceptOption) *testServer {
	t.Helper()
	return start(t, httptest.NewTLSServer, "wss", options...)
}

func start(
	t testing.TB,
	newServer func(http.Handler) *httptest.Server,
	scheme string,
	options ...AcceptOption,
) *testServer {
	t.Helper()

	s := &testServer{
		Headers: make(chan http.Header, 10),
		Dialers: make(chan *Dialer, 10),
		Errors:  make(chan error, 10),
	}

	s.Server = newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Headers <- r.Header.Clone()

		d, err := Accept(w, r, options...)
		if err != nil {
			s.Errors <- err
			return
		}

		t.Cleanup(func() { d.Close() })
		s.Dialers <- d
	}))
	t.Cleanup(s.Server.Close)

	_, rest, _ := strings.Cut(s.Server.URL, "://")
	s.URL = scheme + "://" + rest

	return s
}

// connect calls [Connect], and closes the [Client] when the test ends.
func connect(t testing.TB, url string, options ...ConnectOption) *Client {
	t.Helper()

	c, err := Connect(url, options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	return c
}

// tunnelRecorder is a [ClientObserver] that sends each call to a channel.
type tunnelRecorder struct {
	Opened chan struct{}
	Ended  chan error
	Failed chan error
}

var _ ClientObserver = &tunnelRecorder{}

func newTunnelRecorder() *tunnelRecorder {
	return &tunnelRecorder{
		Opened: make(chan struct{}, 10),
		Ended:  make(chan error, 10),
		Failed: make(chan error, 10),
	}
}

func (r *tunnelRecorder) TunnelOpened()          { r.Opened <- struct{}{} }
func (r *tunnelRecorder) TunnelEnded(err error)  { r.Ended <- err }
func (r *tunnelRecorder) TunnelFailed(err error) { r.Failed <- err }

// receive returns the next value from ch, or fails the test if none arrives
// soon.
func receive[T any](t testing.TB, ch <-chan T) T {
	t.Helper()

	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a value from a channel")
		panic("unreachable")
	}
}

// expectPanic fails the test unless fn panics.
func expectPanic(t testing.TB, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()

	fn()
}

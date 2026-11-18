package tenet_test

import (
	"errors"
	"io"
	"net"
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

// openTunnel opens a tunnel between a new [Client] and a new server, and
// returns the [Client] and the [Dialer] at the two ends.
func openTunnel(t testing.TB, options ...ConnectOption) (*Client, *Dialer) {
	t.Helper()

	s := startServer(t)
	obs := newTunnelRecorder()

	c := connect(t, s.URL, append(options, WithClientObserver(obs))...)
	d := receive(t, s.Dialers)
	receive(t, obs.Opened)

	return c, d
}

// listen calls [Client.Listen], and fails the test if it returns an error.
func listen(t testing.TB, c *Client, network, address string) *Listener {
	t.Helper()

	l, err := c.Listen(network, address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })

	return l
}

// dial opens a connection with d, and returns the [Dialer]'s end of it, and
// the [Listener]'s end that l accepts.
func dial(t testing.TB, d *Dialer, l *Listener) (dialerEnd, listenerEnd net.Conn) {
	t.Helper()

	a := l.Addr()

	dialerEnd, err := d.DialContext(t.Context(), a.Network(), a.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dialerEnd.Close() })

	listenerEnd, err = l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listenerEnd.Close() })

	return dialerEnd, listenerEnd
}

// expectEnd fails the test unless reading from r reaches its end, with
// [io.EOF] or another error, soon.
func expectEnd(t testing.TB, r interface {
	io.Reader
	SetReadDeadline(time.Time) error
}) {
	t.Helper()

	if err := r.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}

	_, err := io.Copy(io.Discard, r)

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatal("timed out waiting for the end of the stream")
	}
}

// expectConnected fails the test unless bytes written to each of a and b
// arrive at the other.
func expectConnected(t testing.TB, a, b net.Conn) {
	t.Helper()

	for _, pair := range [][2]net.Conn{{a, b}, {b, a}} {
		from, to := pair[0], pair[1]

		if _, err := from.Write([]byte("ping")); err != nil {
			t.Fatal(err)
		}

		if err := to.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, 4)
		if _, err := io.ReadFull(to, buf); err != nil {
			t.Fatal(err)
		}

		if err := to.SetReadDeadline(time.Time{}); err != nil {
			t.Fatal(err)
		}
	}
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

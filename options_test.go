package tenet_test

import (
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	. "github.com/dogmatiq/tenet"
)

func TestWithHTTPClient(t *testing.T) {
	t.Run("it makes each attempt to connect with the client", func(t *testing.T) {
		s := startServer(t)

		connect(t, s.URL, WithHTTPClient(clientWithHeader("X-Client", "1")))

		h := receive(t, s.Headers)
		if got, want := h.Get("X-Client"), "1"; got != want {
			t.Fatalf("got X-Client header %q, want %q", got, want)
		}

		receive(t, s.Dialers)
	})

	t.Run("when it is given more than once", func(t *testing.T) {
		t.Run("it uses the last client", func(t *testing.T) {
			s := startServer(t)

			connect(
				t,
				s.URL,
				WithHTTPClient(clientWithHeader("X-Client", "1")),
				WithHTTPClient(clientWithHeader("X-Client", "2")),
			)

			h := receive(t, s.Headers)
			if got, want := h.Get("X-Client"), "2"; got != want {
				t.Fatalf("got X-Client header %q, want %q", got, want)
			}
		})
	})

	t.Run("it panics if c is nil", func(t *testing.T) {
		expectPanic(t, func() {
			WithHTTPClient(nil)
		})
	})
}

func TestWithHeader(t *testing.T) {
	t.Run("it sends the headers with each attempt to connect", func(t *testing.T) {
		s := startServer(t)

		connect(t, s.URL, WithHeader(http.Header{"X-Test": {"1"}}))

		h := receive(t, s.Headers)
		if got, want := h.Get("X-Test"), "1"; got != want {
			t.Fatalf("got X-Test header %q, want %q", got, want)
		}

		receive(t, s.Dialers)
	})

	t.Run("it copies h", func(t *testing.T) {
		s := startServer(t)

		h := http.Header{"X-Test": {"1"}}
		opt := WithHeader(h)
		h.Set("X-Test", "2")

		connect(t, s.URL, opt)

		got := receive(t, s.Headers)
		if got, want := got.Get("X-Test"), "1"; got != want {
			t.Fatalf("got X-Test header %q, want %q", got, want)
		}
	})

	t.Run("it can't replace the headers that the upgrade needs", func(t *testing.T) {
		s := startServer(t)
		obs := newTunnelRecorder()

		connect(
			t,
			s.URL,
			WithHeader(http.Header{
				"Connection":             {"close"},
				"Upgrade":                {"h2c"},
				"Sec-Websocket-Key":      {"x"},
				"Sec-Websocket-Version":  {"1"},
				"Sec-Websocket-Protocol": {"tenet.v0"},
			}),
			WithClientObserver(obs),
		)

		receive(t, s.Dialers)
		receive(t, obs.Opened)
	})

	t.Run("when it is given more than once", func(t *testing.T) {
		t.Run("it uses the last headers", func(t *testing.T) {
			s := startServer(t)

			connect(
				t,
				s.URL,
				WithHeader(http.Header{"X-First": {"1"}}),
				WithHeader(http.Header{"X-Last": {"1"}}),
			)

			h := receive(t, s.Headers)
			if h.Get("X-First") != "" {
				t.Fatal("got the X-First header from the first option")
			}
			if got, want := h.Get("X-Last"), "1"; got != want {
				t.Fatalf("got X-Last header %q, want %q", got, want)
			}
		})
	})
}

func TestWithClientObserver(t *testing.T) {
	t.Run("when it is given more than once", func(t *testing.T) {
		t.Run("it uses the last observer", func(t *testing.T) {
			s := startServer(t)
			first := newTunnelRecorder()
			last := newTunnelRecorder()

			connect(
				t,
				s.URL,
				WithClientObserver(first),
				WithClientObserver(last),
			)

			receive(t, last.Opened)

			if len(first.Opened) != 0 {
				t.Fatal("the first observer was called")
			}
		})
	})

	t.Run("it panics if o is nil", func(t *testing.T) {
		expectPanic(t, func() {
			WithClientObserver(nil)
		})
	})
}

func TestWithBackoff(t *testing.T) {
	t.Run("when it is given more than once", func(t *testing.T) {
		t.Run("it uses the last policy", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := startMemoryServer(t, refuse(nil))
				calls := make(chan string, 10)

				connect(
					t,
					s.URL,
					WithHTTPClient(s.Client),
					WithBackoff(func(int, error) time.Duration {
						calls <- "first"
						return time.Second
					}),
					WithBackoff(func(int, error) time.Duration {
						calls <- "last"
						return time.Second
					}),
				)

				if got := <-calls; got != "last" {
					t.Fatalf("got the %s policy, want the last", got)
				}
			})
		})
	})

	t.Run("it panics if f is nil", func(t *testing.T) {
		expectPanic(t, func() {
			WithBackoff(nil)
		})
	})
}

// clientWithHeader returns an [http.Client] that adds a header to each request
// that it sends.
func clientWithHeader(name, value string) *http.Client {
	return &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			r.Header.Set(name, value)
			return http.DefaultTransport.RoundTrip(r)
		}),
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

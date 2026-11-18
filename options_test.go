package tenet_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
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

	t.Run("when the client follows redirects", func(t *testing.T) {
		t.Run("it connects to the URL that it is redirected to", func(t *testing.T) {
			s := startServer(t)
			redirect := startRedirect(t, s.Server.URL)
			obs := newTunnelRecorder()

			connect(t, redirect.URL, WithHTTPClient(&http.Client{}), WithClientObserver(obs))

			receive(t, redirect.Requests)
			receive(t, s.Dialers)
			receive(t, obs.Opened)
		})
	})

	t.Run("when the client refuses redirects", func(t *testing.T) {
		t.Run("it fails the attempt without following the redirect", func(t *testing.T) {
			s := startServer(t)
			redirect := startRedirect(t, s.Server.URL)
			obs := newTunnelRecorder()

			connect(
				t,
				redirect.URL,
				WithHTTPClient(&http.Client{
					CheckRedirect: func(*http.Request, []*http.Request) error {
						return errors.New("redirects are refused")
					},
				}),
				WithClientObserver(obs),
			)

			receive(t, redirect.Requests)
			receive(t, obs.Failed)

			// The http.Client checks a redirect before it sends the request
			// for it, so a failed attempt never reached the server.
			if len(s.Headers) != 0 {
				t.Fatal("the attempt reached the server after the redirect")
			}
		})
	})

	t.Run("it uses the client's proxy settings", func(t *testing.T) {
		s := startServer(t)

		proxied := make(chan string, 1)
		proxy := httptest.NewServer(&httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				// A request to a forward proxy names its target in full,
				// so the proxy sends it on to that URL.
				proxied <- r.In.URL.String()
				r.Out.URL = r.In.URL
			},
		})
		t.Cleanup(proxy.Close)

		proxyURL, err := url.Parse(proxy.URL)
		if err != nil {
			t.Fatal(err)
		}

		obs := newTunnelRecorder()
		connect(
			t,
			s.URL,
			WithHTTPClient(&http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
				},
			}),
			WithClientObserver(obs),
		)

		if got, want := receive(t, proxied), s.Server.URL; !strings.HasPrefix(got, want) {
			t.Fatalf("the proxy got a request for %q, want one for %q", got, want)
		}
		receive(t, s.Dialers)
		receive(t, obs.Opened)
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

func TestWithKeepAliveInterval(t *testing.T) {
	t.Run("when it is given more than once", func(t *testing.T) {
		t.Run("it uses the last interval", func(t *testing.T) {
			s, _, _, obs := openMemoryTunnel(
				t,
				nil,
				WithKeepAliveInterval(time.Hour),
				WithKeepAliveInterval(50*time.Millisecond),
				WithKeepAliveTimeout(50*time.Millisecond),
			)

			s.LoseNetwork()
			start := time.Now()

			receive(t, obs.Ended)
			if got, limit := time.Since(start), 100*time.Millisecond+slack; got > limit {
				t.Fatalf("the Client's end took %s, want at most %s", got, limit)
			}
		})
	})

	t.Run("it panics if d is not positive", func(t *testing.T) {
		for _, d := range []time.Duration{0, -time.Second} {
			expectPanic(t, func() {
				WithKeepAliveInterval(d)
			})
		}
	})
}

func TestWithKeepAliveTimeout(t *testing.T) {
	t.Run("when it is given more than once", func(t *testing.T) {
		t.Run("it uses the last timeout", func(t *testing.T) {
			s, _, _, obs := openMemoryTunnel(
				t,
				nil,
				WithKeepAliveInterval(50*time.Millisecond),
				WithKeepAliveTimeout(time.Hour),
				WithKeepAliveTimeout(50*time.Millisecond),
			)

			s.LoseNetwork()
			start := time.Now()

			receive(t, obs.Ended)
			if got, limit := time.Since(start), 100*time.Millisecond+slack; got > limit {
				t.Fatalf("the Client's end took %s, want at most %s", got, limit)
			}
		})
	})

	t.Run("it panics if d is not positive", func(t *testing.T) {
		for _, d := range []time.Duration{0, -time.Second} {
			expectPanic(t, func() {
				WithKeepAliveTimeout(d)
			})
		}
	})
}

// redirectServer is an HTTP server that redirects each request.
type redirectServer struct {
	// URL is the server's URL, with the scheme "ws".
	URL string

	// Requests receives a value for each request.
	Requests chan struct{}
}

// startRedirect starts a [redirectServer] that redirects each request to
// target.
func startRedirect(t testing.TB, target string) *redirectServer {
	t.Helper()

	s := &redirectServer{
		Requests: make(chan struct{}, 10),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Requests <- struct{}{}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)

	s.URL = "ws" + strings.TrimPrefix(srv.URL, "http")

	return s
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

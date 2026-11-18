package tenet_test

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	. "github.com/dogmatiq/tenet"
)

func TestConnect(t *testing.T) {
	t.Run("when the URL is invalid", func(t *testing.T) {
		cases := []struct {
			Name string
			URL  string
		}{
			{"unparseable", "ws://example.com/%zz"},
			{"no scheme", "example.com/tunnel"},
			{"relative", "/tunnel"},
			{"http scheme", "http://example.com/tunnel"},
			{"https scheme", "https://example.com/tunnel"},
			{"other scheme", "ftp://example.com/tunnel"},
		}

		for _, tc := range cases {
			t.Run(tc.Name, func(t *testing.T) {
				c, err := Connect(tc.URL)
				if err == nil {
					c.Close()
					t.Fatal("expected an error")
				}
			})
		}
	})

	t.Run("it opens a tunnel with the ws scheme", func(t *testing.T) {
		s := startServer(t)
		obs := newTunnelRecorder()

		connect(t, s.URL, WithClientObserver(obs))

		receive(t, s.Dialers)
		receive(t, obs.Opened)
	})

	t.Run("it opens a tunnel with the wss scheme", func(t *testing.T) {
		s := startTLSServer(t)
		obs := newTunnelRecorder()

		connect(
			t,
			s.URL,
			WithHTTPClient(s.Server.Client()),
			WithClientObserver(obs),
		)

		receive(t, s.Dialers)
		receive(t, obs.Opened)
	})
}

func TestClient(t *testing.T) {
	t.Run("func Listen()", func(t *testing.T) {
		t.Run("when the Client already has a Listener for the address", func(t *testing.T) {
			t.Run("it returns an error that wraps EADDRINUSE", func(t *testing.T) {
				c, _ := openTunnel(t)
				listen(t, c, "tcp", "api.internal:443")

				_, err := c.Listen("tcp", "api.internal:443")
				if !errors.Is(err, syscall.EADDRINUSE) {
					t.Fatalf("got %v, want an error that wraps EADDRINUSE", err)
				}
			})
		})

		t.Run("when the network is the same and the address differs", func(t *testing.T) {
			t.Run("it returns a separate Listener", func(t *testing.T) {
				c, d := openTunnel(t)
				listen(t, c, "tcp", "api.internal:443")
				l := listen(t, c, "tcp", "api.internal:80")

				dial(t, d, l)
			})
		})

		t.Run("when the address is the same and the network differs", func(t *testing.T) {
			t.Run("it returns a separate Listener", func(t *testing.T) {
				c, d := openTunnel(t)
				listen(t, c, "tcp", "api.internal:443")
				l := listen(t, c, "udp", "api.internal:443")

				dial(t, d, l)
			})
		})

		t.Run("when the network or address is too long", func(t *testing.T) {
			cases := []struct {
				Name    string
				Network string
				Address string
			}{
				{"network", strings.Repeat("n", 65), "api.internal:443"},
				{"address", "tcp", strings.Repeat("a", 449)},
			}

			for _, tc := range cases {
				t.Run(tc.Name, func(t *testing.T) {
					c, _ := openTunnel(t)

					_, err := c.Listen(tc.Network, tc.Address)
					if !errors.Is(err, syscall.EINVAL) {
						t.Fatalf("got %v, want an error that wraps EINVAL", err)
					}
				})
			}
		})

		t.Run("when the network and address are as long as they may be", func(t *testing.T) {
			t.Run("it returns a Listener", func(t *testing.T) {
				c, d := openTunnel(t)
				l := listen(t, c, strings.Repeat("n", 64), strings.Repeat("a", 448))

				dial(t, d, l)
			})
		})

		t.Run("when the Client is closed", func(t *testing.T) {
			t.Run("it returns net.ErrClosed", func(t *testing.T) {
				c, _ := openTunnel(t)
				c.Close()

				_, err := c.Listen("tcp", "api.internal:443")
				if err != net.ErrClosed {
					t.Fatalf("got %v, want net.ErrClosed", err)
				}
			})
		})
	})

	t.Run("func Close()", func(t *testing.T) {
		t.Run("it ends the tunnel", func(t *testing.T) {
			s := startServer(t)
			obs := newTunnelRecorder()
			c := connect(t, s.URL, WithClientObserver(obs))

			d := receive(t, s.Dialers)
			receive(t, obs.Opened)

			if err := c.Close(); err != nil {
				t.Fatal(err)
			}

			err := receive(t, obs.Ended)
			if !errors.Is(err, net.ErrClosed) {
				t.Fatalf("got %v, want an error that wraps net.ErrClosed", err)
			}

			receive(t, d.Done())
		})

		t.Run("when a connection is open", func(t *testing.T) {
			t.Run("it leaves the connection open, and ends the tunnel after it closes", func(t *testing.T) {
				s := startServer(t)
				obs := newTunnelRecorder()
				c := connect(t, s.URL, WithClientObserver(obs))
				d := receive(t, s.Dialers)
				receive(t, obs.Opened)

				l := listen(t, c, "tcp", "api.internal:443")
				dialerEnd, listenerEnd := dial(t, d, l)

				c.Close()

				// The tunnel still carries the connection after Close.
				expectConnected(t, dialerEnd, listenerEnd)

				listenerEnd.Close()

				err := receive(t, obs.Ended)
				if !errors.Is(err, net.ErrClosed) {
					t.Fatalf("got %v, want an error that wraps net.ErrClosed", err)
				}

				receive(t, d.Done())
			})
		})

		t.Run("when a connection waits to be accepted", func(t *testing.T) {
			t.Run("it closes the connection, and ends the tunnel", func(t *testing.T) {
				s := startServer(t)
				obs := newTunnelRecorder()
				c := connect(t, s.URL, WithClientObserver(obs))
				d := receive(t, s.Dialers)
				receive(t, obs.Opened)

				listen(t, c, "tcp", "api.internal:443")

				dialerEnd, err := d.DialContext(t.Context(), "tcp", "api.internal:443")
				if err != nil {
					t.Fatal(err)
				}
				defer dialerEnd.Close()

				c.Close()

				expectEnd(t, dialerEnd)

				err = receive(t, obs.Ended)
				if !errors.Is(err, net.ErrClosed) {
					t.Fatalf("got %v, want an error that wraps net.ErrClosed", err)
				}
			})
		})

		t.Run("when the server dials at the same time", func(t *testing.T) {
			t.Run("it ends the tunnel after the last connection closes", func(t *testing.T) {
				s := startServer(t)
				obs := newTunnelRecorder()
				c := connect(t, s.URL, WithClientObserver(obs))
				d := receive(t, s.Dialers)
				receive(t, obs.Opened)

				l := listen(t, c, "tcp", "api.internal:443")

				// Close each connection as soon as it is accepted.
				go func() {
					for {
						conn, err := l.Accept()
						if err != nil {
							return
						}
						conn.Close()
					}
				}()

				var wg sync.WaitGroup
				for range 10 {
					wg.Go(func() {
						for {
							conn, err := d.DialContext(t.Context(), "tcp", "api.internal:443")
							if err != nil {
								return
							}
							conn.Close()
						}
					})
				}

				c.Close()

				err := receive(t, obs.Ended)
				if !errors.Is(err, net.ErrClosed) {
					t.Fatalf("got %v, want an error that wraps net.ErrClosed", err)
				}

				receive(t, d.Done())
				wg.Wait()
			})
		})

		t.Run("it can be called more than once", func(t *testing.T) {
			s := startServer(t)
			obs := newTunnelRecorder()
			c := connect(t, s.URL, WithClientObserver(obs))

			receive(t, obs.Opened)

			for range 2 {
				if err := c.Close(); err != nil {
					t.Fatal(err)
				}
			}

			err := receive(t, obs.Ended)
			if !errors.Is(err, net.ErrClosed) {
				t.Fatalf("got %v, want an error that wraps net.ErrClosed", err)
			}
		})
	})
}

func TestReconnecting(t *testing.T) {
	t.Run("when an attempt to connect fails", func(t *testing.T) {
		t.Run("it calls TunnelFailed with why it failed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := startMemoryServer(t, refuse(nil))
				obs := newTunnelRecorder()

				connect(t, s.URL, WithHTTPClient(s.Client), WithClientObserver(obs))

				if err := <-obs.Failed; err == nil {
					t.Fatal("got nil, want an error")
				}
			})
		})

		t.Run("it connects again after the duration that the backoff policy returns", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts := make(chan time.Time, 10)
				s := startMemoryServer(t, refuse(attempts))

				connect(
					t,
					s.URL,
					WithHTTPClient(s.Client),
					WithBackoff(func(int, error) time.Duration { return 5 * time.Second }),
				)

				prev := <-attempts
				for range 3 {
					next := <-attempts
					if got, want := next.Sub(prev), 5*time.Second; got != want {
						t.Fatalf("got %s between attempts, want %s", got, want)
					}
					prev = next
				}
			})
		})
	})

	t.Run("when the backoff policy returns zero or less", func(t *testing.T) {
		t.Run("it connects again immediately", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts := make(chan time.Time, 10)
				s := startMemoryServer(t, refuse(attempts))

				connect(
					t,
					s.URL,
					WithHTTPClient(s.Client),
					WithBackoff(func(n int, _ error) time.Duration {
						if n%2 == 0 {
							return 0
						}
						return -time.Second
					}),
				)

				first := <-attempts
				for range 4 {
					if got := (<-attempts).Sub(first); got != 0 {
						t.Fatalf("got %s after the first attempt, want 0", got)
					}
				}
			})
		})
	})

	// These tests open tunnels, so they run in real time, outside a synctest
	// bubble. yamux keeps a pool of timers that all sessions share, and a
	// timer made inside a bubble must never be used outside it.

	t.Run("it counts the calls to the backoff policy since a tunnel last opened", func(t *testing.T) {
		var accepting atomic.Bool
		dialers := make(chan *Dialer, 10)

		s := startMemoryServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !accepting.Load() {
				http.Error(w, "not yet", http.StatusServiceUnavailable)
				return
			}

			d, err := Accept(w, r)
			if err != nil {
				t.Error(err)
				return
			}
			t.Cleanup(func() { d.Close() })
			dialers <- d
		}))

		calls := make(chan int, 100)
		connect(
			t,
			s.URL,
			WithHTTPClient(s.Client),
			WithBackoff(func(n int, _ error) time.Duration {
				calls <- n
				if n == 3 {
					accepting.Store(true)
				}
				return 0
			}),
		)

		for want := 1; want <= 3; want++ {
			if got := receive(t, calls); got != want {
				t.Fatalf("got n = %d, want %d", got, want)
			}
		}

		d := receive(t, dialers)
		d.Close()

		if got := receive(t, calls); got != 1 {
			t.Fatalf("got n = %d after a tunnel opened, want 1", got)
		}
	})

	t.Run("it calls TunnelEnded exactly once for each call to TunnelOpened", func(t *testing.T) {
		var attempts atomic.Int64

		// Refuse every other attempt, and end each tunnel at once.
		s := startMemoryServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempts.Add(1)%2 == 1 {
				http.Error(w, "not this time", http.StatusServiceUnavailable)
				return
			}

			d, err := Accept(w, r)
			if err != nil {
				t.Error(err)
				return
			}
			d.Close()
		}))

		calls := make(chan string, 100)
		connect(
			t,
			s.URL,
			WithHTTPClient(s.Client),
			WithClientObserver(&eventRecorder{Calls: calls}),
			WithBackoff(func(int, error) time.Duration { return 0 }),
		)

		// Check the calls for three tunnels, and the failed attempts around
		// them.
		open := false
		for opened := 0; opened < 3 || open; {
			switch receive(t, calls) {
			case "opened":
				if open {
					t.Fatal("TunnelOpened was called twice without TunnelEnded between")
				}
				open = true
				opened++
			case "ended":
				if !open {
					t.Fatal("TunnelEnded was called without TunnelOpened before it")
				}
				open = false
			case "failed":
				if open {
					t.Fatal("TunnelFailed was called while a tunnel was open")
				}
			}
		}
	})

	t.Run("it calls the observer one call at a time, and waits for each call", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var attempts atomic.Int64
			s := startMemoryServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				http.Error(w, "not this time", http.StatusServiceUnavailable)
			}))

			entered := make(chan struct{}, 10)
			release := make(chan struct{})
			obs := &eventRecorder{
				OnFailed: func() {
					entered <- struct{}{}
					<-release
				},
			}

			connect(
				t,
				s.URL,
				WithHTTPClient(s.Client),
				WithClientObserver(obs),
				WithBackoff(func(int, error) time.Duration { return time.Second }),
			)

			<-entered

			// However long TunnelFailed blocks, the Client makes no other
			// attempt.
			time.Sleep(time.Hour)
			synctest.Wait()

			if got := attempts.Load(); got != 1 {
				t.Fatalf("got %d attempts while TunnelFailed blocked, want 1", got)
			}

			close(release)
			<-entered

			if obs.Overlapped() {
				t.Fatal("the observer's calls overlapped")
			}
		})
	})

	t.Run("it calls an observer that two clients share", func(t *testing.T) {
		s := startServer(t)
		obs := newTunnelRecorder()

		a := connect(t, s.URL, WithClientObserver(obs))
		b := connect(t, s.URL, WithClientObserver(obs))

		receive(t, obs.Opened)
		receive(t, obs.Opened)

		a.Close()
		b.Close()

		receive(t, obs.Ended)
		receive(t, obs.Ended)
	})

	t.Run("when the Client is closed", func(t *testing.T) {
		t.Run("it makes no more attempts", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var attempts atomic.Int64
				s := startMemoryServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					attempts.Add(1)
					http.Error(w, "not this time", http.StatusServiceUnavailable)
				}))
				obs := newTunnelRecorder()

				c := connect(
					t,
					s.URL,
					WithHTTPClient(s.Client),
					WithClientObserver(obs),
					WithBackoff(func(int, error) time.Duration { return time.Second }),
				)

				<-obs.Failed
				c.Close()
				synctest.Wait()

				before := attempts.Load()
				time.Sleep(time.Hour)
				synctest.Wait()

				if got := attempts.Load(); got != before {
					t.Fatalf("got %d attempts after Close, want %d", got, before)
				}
			})
		})
	})

	t.Run("when no backoff policy is given", func(t *testing.T) {
		t.Run("it waits 1 second, and doubles the wait up to 1 minute", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts := make(chan time.Time, 20)
				s := startMemoryServer(t, refuse(attempts))

				connect(t, s.URL, WithHTTPClient(s.Client))

				prev := <-attempts
				for _, want := range []time.Duration{
					1 * time.Second,
					2 * time.Second,
					4 * time.Second,
					8 * time.Second,
					16 * time.Second,
					32 * time.Second,
					60 * time.Second,
					60 * time.Second,
				} {
					next := <-attempts
					if got := next.Sub(prev); got < want {
						t.Fatalf("got %s between attempts, want at least %s", got, want)
					}
					prev = next
				}
			})
		})
	})
}

// refuse returns a handler that refuses each request with an HTTP error, and
// sends the time of each request to attempts, if it isn't nil.
func refuse(attempts chan<- time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts != nil {
			attempts <- time.Now()
		}
		http.Error(w, "not this time", http.StatusServiceUnavailable)
	})
}

// eventRecorder is a [ClientObserver] that sends the name of each call to a
// channel, and notices calls that overlap.
type eventRecorder struct {
	// Calls, if it isn't nil, receives "opened", "ended" or "failed" for each
	// call, in order.
	Calls chan<- string

	// OnFailed, if it isn't nil, is called by TunnelFailed.
	OnFailed func()

	inCall     atomic.Bool
	overlapped atomic.Bool
}

var _ ClientObserver = &eventRecorder{}

func (r *eventRecorder) TunnelOpened() {
	r.enter("opened")
	defer r.leave()
}

func (r *eventRecorder) TunnelEnded(error) {
	r.enter("ended")
	defer r.leave()
}

func (r *eventRecorder) TunnelFailed(error) {
	r.enter("failed")
	defer r.leave()

	if r.OnFailed != nil {
		r.OnFailed()
	}
}

// Overlapped returns true if a call started while another was running.
func (r *eventRecorder) Overlapped() bool {
	return r.overlapped.Load()
}

func (r *eventRecorder) enter(name string) {
	if r.inCall.Swap(true) {
		r.overlapped.Store(true)
	}

	if r.Calls != nil {
		// Once a test stops reading, later calls are dropped, so that they
		// can't block the Client.
		select {
		case r.Calls <- name:
		default:
		}
	}
}

func (r *eventRecorder) leave() {
	r.inCall.Store(false)
}

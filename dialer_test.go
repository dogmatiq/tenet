package tenet_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	. "github.com/dogmatiq/tenet"
	"github.com/hashicorp/yamux"
)

func TestAccept(t *testing.T) {
	t.Run("when the client offers a different subprotocol", func(t *testing.T) {
		cases := []struct {
			Name         string
			Subprotocols []string
		}{
			{"none", nil},
			{"another version", []string{"tenet.v0"}},
		}

		for _, tc := range cases {
			t.Run(tc.Name, func(t *testing.T) {
				s := startServer(t)

				ws, res, err := websocket.Dial(
					t.Context(),
					s.URL,
					&websocket.DialOptions{
						Subprotocols: tc.Subprotocols,
					},
				)
				if err == nil {
					ws.CloseNow()
					t.Fatal("expected the upgrade to fail")
				}

				t.Run("it writes an HTTP error response", func(t *testing.T) {
					if res == nil {
						t.Fatal("expected a response")
					}
					if res.StatusCode != http.StatusBadRequest {
						t.Fatalf("got status %d, want %d", res.StatusCode, http.StatusBadRequest)
					}
				})

				t.Run("it returns an error", func(t *testing.T) {
					receive(t, s.Errors)
				})
			})
		}
	})

	t.Run("when the client opens a stream", func(t *testing.T) {
		t.Run("it ends the tunnel", func(t *testing.T) {
			s := startServer(t)
			session := connectRaw(t, s.URL)
			d := receive(t, s.Dialers)

			if _, err := session.OpenStream(); err != nil {
				t.Fatal(err)
			}

			receive(t, d.Done())

			if err := d.Err(); err == nil {
				t.Fatal("got nil, want an error")
			}
		})
	})

	t.Run("when the client offers several subprotocols", func(t *testing.T) {
		t.Run("it chooses its own", func(t *testing.T) {
			s := startServer(t)

			ws, _, err := websocket.Dial(
				t.Context(),
				s.URL,
				&websocket.DialOptions{
					Subprotocols: []string{"tenet.v0", "tenet.v1", "tenet.v2"},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			defer ws.CloseNow()

			if got, want := ws.Subprotocol(), "tenet.v1"; got != want {
				t.Fatalf("got subprotocol %q, want %q", got, want)
			}

			receive(t, s.Dialers)
		})
	})
}

func TestDialer(t *testing.T) {
	t.Run("func DialContext()", func(t *testing.T) {
		t.Run("it returns the Dialer's end of a connection", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			dialerEnd, listenerEnd := dial(t, d, l)

			if _, err := listenerEnd.Write([]byte("pong")); err != nil {
				t.Fatal(err)
			}

			buf := make([]byte, 4)
			if _, err := io.ReadFull(dialerEnd, buf); err != nil {
				t.Fatal(err)
			}
			if got, want := string(buf), "pong"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})

		t.Run("it returns connections with the dialed address as their remote address", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			dialerEnd, _ := dial(t, d, l)

			remote := dialerEnd.RemoteAddr()
			if remote.Network() != "tcp" || remote.String() != "api.internal:443" {
				t.Fatalf("got remote address %q %q, want %q %q", remote.Network(), remote.String(), "tcp", "api.internal:443")
			}

			local := dialerEnd.LocalAddr()
			if local.Network() != "tcp" || local.String() != "" {
				t.Fatalf("got local address %q %q, want %q %q", local.Network(), local.String(), "tcp", "")
			}
		})

		t.Run("it can be used as the dial function of an http.Transport", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:80")

			go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				io.WriteString(w, "hello")
			}))

			client := &http.Client{
				Transport: &http.Transport{
					DialContext: d.DialContext,
				},
			}

			res, err := client.Get("http://api.internal/")
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(body), "hello"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})

		t.Run("it opens connections that don't block each other", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			stalled, _ := dial(t, d, l)
			dialerEnd, listenerEnd := dial(t, d, l)

			// Nothing reads the stalled connection, so this write blocks once
			// the connection's buffer is full.
			go stalled.Write(make([]byte, 1<<20))

			for range 10 {
				if _, err := dialerEnd.Write([]byte("ping")); err != nil {
					t.Fatal(err)
				}

				buf := make([]byte, 4)
				if _, err := io.ReadFull(listenerEnd, buf); err != nil {
					t.Fatal(err)
				}
			}
		})

		t.Run("it opens and closes connections repeatedly through one tunnel", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			for range 500 {
				dialerEnd, err := d.DialContext(t.Context(), "tcp", "api.internal:443")
				if err != nil {
					t.Fatal(err)
				}

				listenerEnd, err := l.Accept()
				if err != nil {
					t.Fatal(err)
				}

				if _, err := dialerEnd.Write([]byte("x")); err != nil {
					t.Fatal(err)
				}
				if _, err := io.ReadFull(listenerEnd, make([]byte, 1)); err != nil {
					t.Fatal(err)
				}

				dialerEnd.Close()
				listenerEnd.Close()
			}
		})

		t.Run("it opens a connection after the handler that called Accept has returned", func(t *testing.T) {
			type accepted struct {
				Dialer  *Dialer
				Request context.Context
			}
			results := make(chan accepted, 1)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				d, err := Accept(w, r)
				if err != nil {
					return
				}
				results <- accepted{d, r.Context()}
			}))
			t.Cleanup(srv.Close)

			c := connect(t, "ws"+strings.TrimPrefix(srv.URL, "http"))
			l := listen(t, c, "tcp", "api.internal:443")

			res := receive(t, results)
			t.Cleanup(func() { res.Dialer.Close() })

			// The request's context ends when the handler returns.
			receive(t, res.Request.Done())

			dial(t, res.Dialer, l)
		})

		t.Run("when no Listener has the address", func(t *testing.T) {
			t.Run("it returns an error that wraps ECONNREFUSED", func(t *testing.T) {
				c, d := openTunnel(t)
				listen(t, c, "tcp", "api.internal:443")

				_, err := d.DialContext(t.Context(), "tcp", "api.internal:80")
				if !errors.Is(err, syscall.ECONNREFUSED) {
					t.Fatalf("got %v, want an error that wraps ECONNREFUSED", err)
				}
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
					_, d := openTunnel(t)

					_, err := d.DialContext(t.Context(), tc.Network, tc.Address)
					if !errors.Is(err, syscall.EINVAL) {
						t.Fatalf("got %v, want an error that wraps EINVAL", err)
					}
				})
			}
		})

		t.Run("when ctx ends while it waits for the client", func(t *testing.T) {
			s := startServer(t)
			session := connectRaw(t, s.URL)
			d := receive(t, s.Dialers)

			// Accept each stream, but never answer it.
			streams := make(chan *yamux.Stream, 1)
			go func() {
				if stream, err := session.AcceptStream(); err == nil {
					streams <- stream
				}
			}()

			ctx, cancel := context.WithCancel(t.Context())
			errs := make(chan error, 1)
			go func() {
				_, err := d.DialContext(ctx, "tcp", "api.internal:443")
				errs <- err
			}()

			stream := receive(t, streams)
			cancel()

			t.Run("it returns an error that wraps ctx.Err()", func(t *testing.T) {
				err := receive(t, errs)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("got %v, want an error that wraps context.Canceled", err)
				}
			})

			t.Run("it closes the stream", func(t *testing.T) {
				expectEnd(t, stream)
			})
		})

		t.Run("when the tunnel has ended", func(t *testing.T) {
			t.Run("it returns an error that wraps the error from Err", func(t *testing.T) {
				_, d := openTunnel(t)
				d.Close()

				_, err := d.DialContext(t.Context(), "tcp", "api.internal:443")
				if !errors.Is(err, net.ErrClosed) {
					t.Fatalf("got %v, want an error that wraps net.ErrClosed", err)
				}
			})
		})
	})

	t.Run("when the tunnel ends", func(t *testing.T) {
		cases := []struct {
			Name string
			End  func(*Dialer, net.Conn)
		}{
			{
				"the Dialer is closed",
				func(d *Dialer, _ net.Conn) { d.Close() },
			},
			{
				"the client's network connection breaks",
				func(_ *Dialer, raw net.Conn) { raw.Close() },
			},
		}

		for _, tc := range cases {
			t.Run(tc.Name, func(t *testing.T) {
				t.Run("it ends each connection at both ends", func(t *testing.T) {
					var raw net.Conn
					var m sync.Mutex

					c, d := openTunnel(
						t,
						WithHTTPClient(&http.Client{
							Transport: &http.Transport{
								DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
									conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
									m.Lock()
									raw = conn
									m.Unlock()
									return conn, err
								},
							},
						}),
					)
					l := listen(t, c, "tcp", "api.internal:443")
					dialerEnd, listenerEnd := dial(t, d, l)

					m.Lock()
					tc.End(d, raw)
					m.Unlock()

					expectEnd(t, dialerEnd)
					expectEnd(t, listenerEnd)

					receive(t, d.Done())
				})
			})
		}
	})

	t.Run("when one end of a connection closes it", func(t *testing.T) {
		t.Run("it ends the other end", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			dialerEnd, listenerEnd := dial(t, d, l)
			dialerEnd.Close()
			expectEnd(t, listenerEnd)

			dialerEnd, listenerEnd = dial(t, d, l)
			listenerEnd.Close()
			expectEnd(t, dialerEnd)
		})
	})

	t.Run("func Err()", func(t *testing.T) {
		t.Run("it returns nil until the channel from Done is closed", func(t *testing.T) {
			s := startServer(t)
			obs := newTunnelRecorder()
			c := connect(t, s.URL, WithClientObserver(obs))

			d := receive(t, s.Dialers)
			receive(t, obs.Opened)

			if err := d.Err(); err != nil {
				t.Fatalf("got %v, want nil", err)
			}

			c.Close()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			for {
				// Done is checked after Err, so if Done isn't closed yet, it
				// wasn't closed when Err was called either.
				err := d.Err()

				select {
				case <-d.Done():
					if d.Err() == nil {
						t.Fatal("got nil after Done was closed, want an error")
					}
					return
				case <-ctx.Done():
					t.Fatal("timed out waiting for the tunnel to end")
				default:
					if err != nil {
						t.Fatalf("got %v before Done was closed, want nil", err)
					}
				}
			}
		})
	})

	t.Run("func Close()", func(t *testing.T) {
		t.Run("it ends the tunnel with net.ErrClosed", func(t *testing.T) {
			s := startServer(t)
			obs := newTunnelRecorder()
			connect(t, s.URL, WithClientObserver(obs))

			d := receive(t, s.Dialers)
			receive(t, obs.Opened)

			if err := d.Close(); err != nil {
				t.Fatal(err)
			}

			select {
			case <-d.Done():
			default:
				t.Fatal("expected Done to be closed")
			}

			if err := d.Err(); err != net.ErrClosed {
				t.Fatalf("got %v, want net.ErrClosed", err)
			}

			if err := receive(t, obs.Ended); err == nil {
				t.Fatal("got nil, want an error")
			}
		})

		t.Run("it can be called more than once", func(t *testing.T) {
			s := startServer(t)
			connect(t, s.URL)

			d := receive(t, s.Dialers)

			for range 2 {
				if err := d.Close(); err != nil {
					t.Fatal(err)
				}
			}

			if err := d.Err(); err != net.ErrClosed {
				t.Fatalf("got %v, want net.ErrClosed", err)
			}
		})
	})

	t.Run("when the client closes the tunnel", func(t *testing.T) {
		t.Run("it ends the tunnel", func(t *testing.T) {
			s := startServer(t)
			obs := newTunnelRecorder()
			c := connect(t, s.URL, WithClientObserver(obs))

			d := receive(t, s.Dialers)
			receive(t, obs.Opened)

			c.Close()

			receive(t, d.Done())

			if err := d.Err(); err == nil {
				t.Fatal("got nil, want an error")
			}
		})
	})
}

// connectRaw connects to the server at url as a client made from the
// WebSocket and yamux libraries directly, so that it can break the rules that
// tenet's own client keeps. It returns the client's yamux session.
func connectRaw(t testing.TB, url string) *yamux.Session {
	t.Helper()

	ws, _, err := websocket.Dial(
		t.Context(),
		url,
		&websocket.DialOptions{
			Subprotocols: []string{"tenet.v1"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard

	session, err := yamux.Server(
		websocket.NetConn(context.Background(), ws, websocket.MessageBinary),
		cfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })

	return session
}

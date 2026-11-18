package tenet_test

import (
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"

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

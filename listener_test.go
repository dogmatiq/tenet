package tenet_test

import (
	"errors"
	"io"
	"net"
	"syscall"
	"testing"

	. "github.com/dogmatiq/tenet"
)

func TestListener(t *testing.T) {
	t.Run("func Accept()", func(t *testing.T) {
		t.Run("it returns the Listener's end of each connection", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			dialerEnd, listenerEnd := dial(t, d, l)

			if _, err := dialerEnd.Write([]byte("ping")); err != nil {
				t.Fatal(err)
			}

			buf := make([]byte, 4)
			if _, err := io.ReadFull(listenerEnd, buf); err != nil {
				t.Fatal(err)
			}
			if got, want := string(buf), "ping"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})

		t.Run("when the Listener is closed", func(t *testing.T) {
			t.Run("it returns net.ErrClosed", func(t *testing.T) {
				c, _ := openTunnel(t)
				l := listen(t, c, "tcp", "api.internal:443")

				errs := make(chan error, 1)
				go func() {
					_, err := l.Accept()
					errs <- err
				}()

				l.Close()

				if err := receive(t, errs); err != net.ErrClosed {
					t.Fatalf("got %v, want net.ErrClosed", err)
				}

				if _, err := l.Accept(); err != net.ErrClosed {
					t.Fatalf("got %v, want net.ErrClosed", err)
				}
			})
		})

		t.Run("when the Client is closed", func(t *testing.T) {
			t.Run("it returns net.ErrClosed", func(t *testing.T) {
				c, _ := openTunnel(t)
				l := listen(t, c, "tcp", "api.internal:443")

				c.Close()

				if _, err := l.Accept(); err != net.ErrClosed {
					t.Fatalf("got %v, want net.ErrClosed", err)
				}
			})
		})

		t.Run("when the tunnel ends", func(t *testing.T) {
			t.Run("it keeps waiting, and returns no error", func(t *testing.T) {
				s := startServer(t)
				obs := newTunnelRecorder()
				c := connect(t, s.URL, WithClientObserver(obs))
				d := receive(t, s.Dialers)
				receive(t, obs.Opened)

				l := listen(t, c, "tcp", "api.internal:443")

				errs := make(chan error, 1)
				go func() {
					_, err := l.Accept()
					errs <- err
				}()

				d.Close()
				receive(t, obs.Ended)

				// Accept returns only once. If the tunnel's end had made it
				// return, it would have returned an error other than
				// net.ErrClosed, because the Listener wasn't closed yet.
				l.Close()

				if err := receive(t, errs); err != net.ErrClosed {
					t.Fatalf("got %v, want net.ErrClosed", err)
				}
			})
		})

		t.Run("it returns connections with the Listener's address as their local address", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			_, listenerEnd := dial(t, d, l)

			local := listenerEnd.LocalAddr()
			if local.Network() != "tcp" || local.String() != "api.internal:443" {
				t.Fatalf("got local address %q %q, want %q %q", local.Network(), local.String(), "tcp", "api.internal:443")
			}

			remote := listenerEnd.RemoteAddr()
			if remote.Network() != "tcp" || remote.String() != "" {
				t.Fatalf("got remote address %q %q, want %q %q", remote.Network(), remote.String(), "tcp", "")
			}
		})
	})

	t.Run("func Close()", func(t *testing.T) {
		t.Run("it causes dials to its address to be refused", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			if err := l.Close(); err != nil {
				t.Fatal(err)
			}

			_, err := d.DialContext(t.Context(), "tcp", "api.internal:443")
			if !errors.Is(err, syscall.ECONNREFUSED) {
				t.Fatalf("got %v, want an error that wraps ECONNREFUSED", err)
			}
		})

		t.Run("it closes the connections that wait to be accepted", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			dialerEnd, err := d.DialContext(t.Context(), "tcp", "api.internal:443")
			if err != nil {
				t.Fatal(err)
			}
			defer dialerEnd.Close()

			l.Close()

			expectEnd(t, dialerEnd)
		})

		t.Run("it leaves open the connections that Accept returned", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			dialerEnd, listenerEnd := dial(t, d, l)

			l.Close()

			expectConnected(t, dialerEnd, listenerEnd)
		})

		t.Run("it can be called more than once", func(t *testing.T) {
			c, _ := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			for range 2 {
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})

		t.Run("it allows Listen to be called again for its address", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")
			l.Close()

			l = listen(t, c, "tcp", "api.internal:443")
			dial(t, d, l)
		})
	})

	t.Run("func Addr()", func(t *testing.T) {
		t.Run("it returns the network and address given to Listen", func(t *testing.T) {
			c, _ := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			a := l.Addr()
			if a.Network() != "tcp" || a.String() != "api.internal:443" {
				t.Fatalf("got %q %q, want %q %q", a.Network(), a.String(), "tcp", "api.internal:443")
			}
		})
	})

	t.Run("when its backlog is full", func(t *testing.T) {
		t.Run("it causes dials to its address to be reset", func(t *testing.T) {
			c, d := openTunnel(t)
			l := listen(t, c, "tcp", "api.internal:443")

			for range 256 {
				conn, err := d.DialContext(t.Context(), "tcp", "api.internal:443")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { conn.Close() })
			}

			_, err := d.DialContext(t.Context(), "tcp", "api.internal:443")
			if !errors.Is(err, syscall.ECONNRESET) {
				t.Fatalf("got %v, want an error that wraps ECONNRESET", err)
			}

			// Accepting one connection makes room for another.
			conn, err := l.Accept()
			if err != nil {
				t.Fatal(err)
			}
			conn.Close()

			conn, err = d.DialContext(t.Context(), "tcp", "api.internal:443")
			if err != nil {
				t.Fatal(err)
			}
			conn.Close()
		})
	})
}

package tenet_test

import (
	"errors"
	"io"
	"syscall"
	"testing"
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

package tenet_test

import (
	"errors"
	"net"
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

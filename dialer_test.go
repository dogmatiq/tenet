package tenet_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	. "github.com/dogmatiq/tenet"
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

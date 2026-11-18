package tenet_test

import (
	"net/http"
	"testing"
	"time"

	. "github.com/dogmatiq/tenet"
)

// These tests run in real time, outside a synctest bubble. yamux keeps a pool
// of timers that all sessions share, and a timer made inside a bubble must
// never be used outside it.

// slack is how much longer than its settings allow a tunnel may take to end,
// for the scheduling of goroutines. It is well under the 10 seconds that a
// WebSocket's close handshake may wait for an answer that never comes.
const slack = 1 * time.Second

func TestLiveness(t *testing.T) {
	t.Run("when the network is lost without a word", func(t *testing.T) {
		t.Run("it ends the tunnel at each end within the interval plus the timeout", func(t *testing.T) {
			s, _, d, obs := openMemoryTunnel(
				t,
				[]AcceptOption{
					WithKeepAliveInterval(50 * time.Millisecond),
					WithKeepAliveTimeout(50 * time.Millisecond),
				},
				WithKeepAliveInterval(50*time.Millisecond),
				WithKeepAliveTimeout(50*time.Millisecond),
			)

			s.LoseNetwork()
			start := time.Now()
			limit := 100*time.Millisecond + slack

			receive(t, d.Done())
			if got := time.Since(start); got > limit {
				t.Fatalf("the Dialer's end took %s, want at most %s", got, limit)
			}
			if d.Err() == nil {
				t.Fatal("got nil from Err, want an error")
			}

			if err := receive(t, obs.Ended); err == nil {
				t.Fatal("got nil from TunnelEnded, want an error")
			}
			if got := time.Since(start); got > limit {
				t.Fatalf("the Client's end took %s, want at most %s", got, limit)
			}
		})

		t.Run("it uses each end's own settings", func(t *testing.T) {
			s, _, d, obs := openMemoryTunnel(
				t,
				[]AcceptOption{
					WithKeepAliveInterval(1 * time.Second),
					WithKeepAliveTimeout(1 * time.Second),
				},
				WithKeepAliveInterval(50*time.Millisecond),
				WithKeepAliveTimeout(50*time.Millisecond),
			)

			s.LoseNetwork()
			start := time.Now()

			receive(t, obs.Ended)
			if got, limit := time.Since(start), 100*time.Millisecond+slack; got > limit {
				t.Fatalf("the Client's end took %s, want at most %s", got, limit)
			}

			receive(t, d.Done())
			got := time.Since(start)
			if least := 1 * time.Second; got < least {
				t.Fatalf("the Dialer's end took %s, want at least its timeout, %s", got, least)
			}
			if limit := 2*time.Second + slack; got > limit {
				t.Fatalf("the Dialer's end took %s, want at most %s", got, limit)
			}
		})

		t.Run("it uses the default settings at each end", func(t *testing.T) {
			if testing.Short() {
				t.Skip("the default settings take up to 25 seconds")
			}

			s, _, d, obs := openMemoryTunnel(t, nil)

			s.LoseNetwork()
			start := time.Now()
			limit := 15*time.Second + 10*time.Second + slack

			select {
			case <-d.Done():
			case <-time.After(limit):
				t.Fatalf("the Dialer's end took more than %s", limit)
			}

			select {
			case <-obs.Ended:
			case <-time.After(limit - time.Since(start)):
				t.Fatalf("the Client's end took more than %s", limit)
			}
		})
	})

	t.Run("when the network stops accepting writes", func(t *testing.T) {
		t.Run("it ends the tunnel within the timeout", func(t *testing.T) {
			s, c, d, obs := openMemoryTunnel(
				t,
				[]AcceptOption{
					WithKeepAliveInterval(time.Hour),
					WithKeepAliveTimeout(50 * time.Millisecond),
				},
				WithKeepAliveInterval(time.Hour),
				WithKeepAliveTimeout(50*time.Millisecond),
			)

			l := listen(t, c, "tcp", "api.internal:443")
			_, listenerEnd := dial(t, d, l)

			s.StallNetwork()
			start := time.Now()

			if _, err := listenerEnd.Write([]byte("ping")); err == nil {
				t.Fatal("got nil from Write, want an error")
			}

			receive(t, obs.Ended)
			if got, limit := time.Since(start), 50*time.Millisecond+slack; got > limit {
				t.Fatalf("the Client's end took %s, want at most %s", got, limit)
			}
		})
	})
}

// openMemoryTunnel opens a tunnel through a [memoryServer] that passes
// acceptOptions to [Accept]. It returns the server, the [Client], the [Dialer],
// and a [tunnelRecorder] that observes the [Client].
func openMemoryTunnel(
	t testing.TB,
	acceptOptions []AcceptOption,
	connectOptions ...ConnectOption,
) (*memoryServer, *Client, *Dialer, *tunnelRecorder) {
	t.Helper()

	dialers := make(chan *Dialer, 1)
	s := startMemoryServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := Accept(w, r, acceptOptions...)
		if err != nil {
			t.Error(err)
			return
		}
		t.Cleanup(func() { d.Close() })
		dialers <- d
	}))

	obs := newTunnelRecorder()
	c := connect(
		t,
		s.URL,
		append(
			connectOptions,
			WithHTTPClient(s.Client),
			WithClientObserver(obs),
			// The Client must not open a new tunnel after the network is
			// lost, so that each test sees only the one tunnel.
			WithBackoff(func(int, error) time.Duration { return time.Hour }),
		)...,
	)

	d := receive(t, dialers)
	receive(t, obs.Opened)

	return s, c, d, obs
}

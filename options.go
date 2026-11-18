package tenet

import (
	"net/http"
	"time"
)

// A ConnectOption is an option that [Connect] takes.
//
// If the same option is given more than once, the last one applies.
type ConnectOption interface {
	applyToConnect()
}

// An AcceptOption is an option that [Accept] takes.
//
// If the same option is given more than once, the last one applies.
type AcceptOption interface {
	applyToAccept()
}

// An Option is both a [ConnectOption] and an [AcceptOption], so [Connect] and
// [Accept] both take it.
type Option interface {
	ConnectOption
	AcceptOption
}

// WithHTTPClient returns a [ConnectOption] that sets the [http.Client] that
// makes each attempt to connect. The client's redirect, proxy and TLS settings
// apply to each attempt as they do to any other request. The default is
// [http.DefaultClient].
//
// It panics if c is nil.
func WithHTTPClient(c *http.Client) ConnectOption {
	panic("not implemented")
}

// WithClientObserver returns a [ConnectOption] that sets the [ClientObserver]
// that learns about the tunnels that the [Client] opens. By default, the
// [Client] has no [ClientObserver].
//
// It panics if o is nil.
func WithClientObserver(o ClientObserver) ConnectOption {
	panic("not implemented")
}

// WithKeepAliveInterval returns an [Option] that sets how often one end of a
// tunnel checks that the other end still answers. It applies to the [Client] if
// it is given to [Connect], and to the [Dialer] if it is given to [Accept]. The
// default is 15 seconds.
//
// It panics if d is not positive.
func WithKeepAliveInterval(d time.Duration) Option {
	panic("not implemented")
}

// WithKeepAliveTimeout returns an [Option] that sets how long the other end of
// a tunnel may take to answer a check, or to accept a write, before the tunnel
// ends. The default is 10 seconds.
//
// It panics if d is not positive.
func WithKeepAliveTimeout(d time.Duration) Option {
	panic("not implemented")
}

// WithBackoff returns a [ConnectOption] that sets the [Client]'s backoff
// policy, which decides how long the [Client] waits before it connects again.
//
// The policy is the function f. The [Client] calls f each time a tunnel ends,
// and each time an attempt to connect fails, with why in err. It waits for the
// duration that f returns, and then connects again. If that duration is zero or
// less, it connects again immediately. n counts the calls to f since a tunnel
// was last opened, starting at 1.
//
// By default, the [Client] waits 1 second, doubles the wait after each call up
// to 1 minute, and adds random jitter to each wait.
//
// It panics if f is nil.
func WithBackoff(f func(n int, err error) time.Duration) ConnectOption {
	panic("not implemented")
}

// WithHeader returns a [ConnectOption] that sets the HTTP headers that each
// attempt to connect sends, in addition to the headers that the [http.Client]
// and the upgrade to a WebSocket add. It copies h, so later changes to h don't
// affect the [Client]. It can't replace the headers that the upgrade needs.
//
// A value that changes between attempts, such as a token, belongs in the
// [http.Client] that [WithHTTPClient] sets.
func WithHeader(h http.Header) ConnectOption {
	panic("not implemented")
}

package tenet

import (
	"net/http"
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

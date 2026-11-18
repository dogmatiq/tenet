package tenet

// A ClientObserver learns about each tunnel that a [Client] opens, and about
// each attempt to open one that fails.
//
// Pass a ClientObserver to [Connect] with [WithClientObserver].
//
// The [Client] calls the methods one at a time, in the order that the events
// happen. A method that blocks delays the [Client]'s next attempt to connect.
// If the same ClientObserver is given to more than one [Client], the calls for
// different [Client] values may run at the same time, so it must be safe for
// concurrent use.
type ClientObserver interface {
	// TunnelOpened is called each time a tunnel opens.
	TunnelOpened()

	// TunnelEnded is called each time a tunnel ends, with why it ended. It is
	// called exactly once for each call to [ClientObserver.TunnelOpened].
	//
	// err wraps [net.ErrClosed] if [Client.Close] ended the tunnel.
	TunnelEnded(err error)

	// TunnelFailed is called each time an attempt to open a tunnel fails, with why
	// it failed.
	TunnelFailed(err error)
}

// nopObserver is a [ClientObserver] that does nothing. A [Client] uses it when
// it's given no other.
type nopObserver struct{}

func (nopObserver) TunnelOpened()      {}
func (nopObserver) TunnelEnded(error)  {}
func (nopObserver) TunnelFailed(error) {}

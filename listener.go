package tenet

import (
	"net"
	"sync"
)

// backlog is the number of connections that may wait for each [Listener] to
// accept them.
const backlog = 256

// A Listener is a [net.Listener] for the connections that the server opens with
// [Dialer.DialContext] for one network and address.
//
// Obtain a Listener by calling [Client.Listen]. A Listener may be used until
// [Listener.Close] or [Client.Close] is called.
type Listener struct {
	client *Client
	addr   addr

	// queue holds the connections that wait for Accept. Its capacity is
	// [backlog], and each send to it uses a slot that reserve took, so it
	// never blocks.
	queue chan *conn

	// closed is closed by Close.
	closed chan struct{}

	m        sync.Mutex
	isClosed bool

	// slots is the number of connections that are in queue or about to be
	// sent to it.
	slots int
}

// reserve takes a slot in the queue for a connection, and returns [accepted].
// It returns [backlogFull] if there is no free slot, and [refused] if the
// [Listener] is closed.
func (l *Listener) reserve() answer {
	l.m.Lock()
	defer l.m.Unlock()

	switch {
	case l.isClosed:
		return refused
	case l.slots == backlog:
		return backlogFull
	default:
		l.slots++
		return accepted
	}
}

// release frees a slot that reserve took.
func (l *Listener) release() {
	l.m.Lock()
	defer l.m.Unlock()
	l.slots--
}

// enqueue queues c for Accept, in a slot that reserve took. If the [Listener]
// has been closed since, it closes c instead.
func (l *Listener) enqueue(c *conn) {
	l.m.Lock()
	defer l.m.Unlock()

	if l.isClosed {
		c.Close()
		return
	}

	l.queue <- c
}

// Accept waits for the next connection that the server opens to the
// [Listener]'s network and address, and returns the [Listener]'s end of it.
//
// It returns [net.ErrClosed] once [Listener.Close] or [Client.Close] is called.
// It returns no other error, so a function such as [http.Server.Serve] that
// serves the [Listener] keeps serving it while the [Client] connects again.
//
// The connection's [net.Conn.LocalAddr] method returns the address that
// [Listener.Addr] returns. Its [net.Conn.RemoteAddr] method returns an address
// with the same network, whose string form is empty.
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case <-l.closed:
		return nil, net.ErrClosed
	case c := <-l.queue:
		l.release()

		// The select chooses at random when both are ready, so a connection
		// may be received after Close.
		select {
		case <-l.closed:
			c.Close()
			return nil, net.ErrClosed
		default:
			return c, nil
		}
	}
}

// Close stops the [Listener] from accepting connections. Connections that the
// server has opened, but that [Listener.Accept] has not returned, are closed.
// Connections that [Listener.Accept] has returned stay open until they are
// closed.
//
// After Close, a call to [Dialer.DialContext] for the [Listener]'s network and
// address returns an error that wraps [syscall.ECONNREFUSED], and
// [Client.Listen] may be called again for that network and address.
//
// Close may be called more than once, and always returns nil.
func (l *Listener) Close() error {
	l.m.Lock()
	if l.isClosed {
		l.m.Unlock()
		return nil
	}
	l.isClosed = true
	close(l.closed)
	l.m.Unlock()

	l.client.remove(l)

	// No connection is queued after isClosed is set, so this closes every
	// connection that is left.
	for {
		select {
		case c := <-l.queue:
			c.Close()
		default:
			return nil
		}
	}
}

// Addr returns the [Listener]'s address. Its [net.Addr.Network] and
// [net.Addr.String] methods return the network and address that were given to
// [Client.Listen].
func (l *Listener) Addr() net.Addr {
	return l.addr
}

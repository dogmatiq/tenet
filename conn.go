package tenet

import (
	"net"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// conn is one end of a connection through a tunnel. It is a [net.Conn].
type conn struct {
	stream *yamux.Stream
	local  addr
	remote addr

	// onClose, if it isn't nil, is called the first time Close is called.
	onClose   func()
	closeOnce sync.Once
}

func (c *conn) Read(b []byte) (int, error) {
	return c.stream.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	return c.stream.Write(b)
}

func (c *conn) Close() error {
	err := c.stream.Close()

	if c.onClose != nil {
		c.closeOnce.Do(c.onClose)
	}

	return err
}

func (c *conn) LocalAddr() net.Addr {
	return c.local
}

func (c *conn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *conn) SetDeadline(t time.Time) error {
	return c.stream.SetDeadline(t)
}

func (c *conn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

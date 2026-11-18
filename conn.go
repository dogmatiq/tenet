package tenet

import (
	"net"
	"time"

	"github.com/hashicorp/yamux"
)

// conn is one end of a connection through a tunnel. It is a [net.Conn].
type conn struct {
	stream *yamux.Stream
	local  addr
	remote addr
}

func (c *conn) Read(b []byte) (int, error) {
	return c.stream.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	return c.stream.Write(b)
}

func (c *conn) Close() error {
	return c.stream.Close()
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

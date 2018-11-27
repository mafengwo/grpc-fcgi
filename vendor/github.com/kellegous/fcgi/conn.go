package fcgi

import (
	"net"
	"time"
)

type conn struct {
	net.Conn
	timeout time.Duration
}

// Write ...
func (c *conn) Write(b []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(b)
}

// Read ...
func (c *conn) Read(b []byte) (int, error) {
	if err := c.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(b)
}

func dial(c *Client) (*conn, error) {
	cn, err := net.DialTimeout(c.net, c.addr, c.dialTimeout)
	if err != nil {
		return nil, err
	}

	return &conn{
		Conn:    cn,
		timeout: c.timeout,
	}, nil
}

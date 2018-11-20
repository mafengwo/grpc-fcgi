package fastcgi

import (
	"io"
	"net"
	"time"
)

type conn struct {
	net.Conn
	timeout time.Duration

	readTimeout  time.Duration
	writeTimeout time.Duration

	stdin  io.Reader
	stderr io.ReadWriteCloser
	stdout io.ReadWriteCloser
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

func dial(proto, addr string, dialTimeout, timeout time.Duration) (*conn, error) {
	cn, err := net.DialTimeout(proto, addr, dialTimeout)
	if err != nil {
		return nil, err
	}

	return &conn{
		Conn:    cn,
		timeout: timeout,
	}, nil
}

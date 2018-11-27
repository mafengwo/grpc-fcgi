package fcgi

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"net/textproto"
	"os"
	"time"
)

const (
	defaultDialTimeout = time.Minute
	defaultTimeout     = time.Minute
)

// Client ...
type Client struct {
	net         string
	addr        string
	dialTimeout time.Duration
	timeout     time.Duration
}

// ClientOption ...
type ClientOption func(*Client) error

func applyOptions(c *Client, options []ClientOption) error {
	for _, opt := range options {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

// NewClient ...
func NewClient(net, addr string, options ...ClientOption) (*Client, error) {
	c := &Client{
		net:         net,
		addr:        addr,
		dialTimeout: defaultDialTimeout,
		timeout:     defaultTimeout,
	}

	if err := applyOptions(c, options); err != nil {
		return nil, err
	}

	return c, nil
}

// WithDialTimeout creates a DialOption that sets the dial timeout.
// The default is 60 seconds.
func WithDialTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) error {
		c.dialTimeout = timeout
		return nil
	}
}

// WithTimeout creates a DialOption that sets the timeout for the client dispatching a record
// to the FCGI backend. The default is 60 seconds.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) error {
		c.timeout = timeout
		return nil
	}
}

// NewRequest ...
func (c *Client) NewRequest(params map[string][]string) (*Request, error) {
	cn, err := dial(c)
	if err != nil {
		return nil, err
	}

	var buf buffer
	buf.Reset()

	if err := writeBeginReq(cn, &buf, requestID); err != nil {
		cn.Close()
		return nil, err
	}

	if err := writeParams(cn, &buf, requestID, params); err != nil {
		cn.Close()
		return nil, err
	}

	return &Request{
		c: cn,
	}, nil
}

// ServeHTTP ...
func (c *Client) ServeHTTP(params map[string][]string,
	w http.ResponseWriter,
	r *http.Request) {

	req, err := c.NewRequest(params)
	if err != nil {
		panic(err)
	}

	pr, pw := io.Pipe()
	req.Stdout = pw
	req.Stderr = os.Stderr
	req.Stdin = r.Body

	go func() {
		if err := req.Wait(); err != nil {
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}()

	br := bufio.NewReader(pr)
	tr := textproto.NewReader(br)
	mh, err := tr.ReadMIMEHeader()
	if err != nil {
		log.Panic(err)
	}

	h := http.Header(mh)
	s, err := statusFromHeaders(h)
	if err != nil {
		log.Panic(err)
	}

	w.WriteHeader(s)
	for key, vals := range h {
		for _, val := range vals {
			w.Header().Add(key, val)
		}
	}

	if _, err := io.Copy(w, br); err != nil {
		log.Panic(err)
	}
}

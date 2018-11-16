package fastcgi

import (
	"bufio"
	"bytes"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strconv"
	"time"
)

type ClientConfig struct {
	Net         string
	Addr        string
	DialTimeout time.Duration
	Timeout     time.Duration
}

// Client ...
type Client struct {
	config *ClientConfig

	conn *conn
}

func NewClient(conf *ClientConfig) (*Client) {
	c := &Client{
		config: conf,
	}
	return c
}

type SizedReader interface {
	io.Reader
	Size() int64
}

func (c *Client) Send(params map[string][]string, body SizedReader) (respHeader http.Header, respBody []byte, err error) {
	if c.conn == nil {
		c.conn, err = dial(c.config.Net, c.config.Addr, c.config.DialTimeout, c.config.Timeout)
		if err != nil {
			return
		}
	}

	var buf buffer
	buf.Reset()

	if err = writeBeginReq(c.conn, &buf, requestID); err != nil {
		//c.conn.Close()
		return
	}

	// if CONTENT_LENGTH is missed or mismatch the body length, the all following
	// request will be ruined. so CONTENT_LENGTH must be override
	params["CONTENT_LENGTH"] = []string{strconv.Itoa(int(body.Size()))}

	if err = writeParams(c.conn, &buf, requestID, params); err != nil {
		//c.conn.Close()
		return
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	req := &Request{
		c:      c.conn,
		Stdin:  body,
		Stdout: stdout,
		Stderr: stderr,
	}
	if err = req.Wait(); err != nil {
		err = errors.WithMessage(err, "failed to wait for FCGI response")
		return
	}

	rb := bufio.NewReader(stdout)
	tp := textproto.NewReader(rb)

	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		err = errors.WithMessage(err, "failed to parse FCGI response header")
		return
	}
	respHeader = http.Header(mimeHeader)

	// TODO: fixTransferEncoding ?
	if chunked(respHeader["Transfer-Encoding"]) {
		respBody, err = ioutil.ReadAll(httputil.NewChunkedReader(rb))
		if err != nil {
			err = errors.WithMessage(err, "failed to parse FCGI chunked response")
			return
		}
	} else {
		respBody, err = ioutil.ReadAll(rb)
		if err != nil {
			err = errors.WithMessage(err, "failed to parse FCGI response")
			return
		}
	}
	return
}

func chunked(te []string) bool { return len(te) > 0 && te[0] == "chunked" }

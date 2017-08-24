package proxy

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"math"
	"net/http"
	"net/textproto"
	"strconv"
	"sync"

	"github.com/kellegous/fcgi"
	"github.com/pkg/errors"
)

type client struct {
	sync.Mutex
	max    int
	addr   string
	client *fcgi.Client
	server *Server
}

type fastcgiResponse struct {
	code   int
	body   []byte
	header http.Header
}

func newClient(s *Server, addr string, num int) *client {
	p := &client{
		addr:   addr,
		max:    num,
		server: s,
	}

	if p.max > math.MaxUint16 {
		p.max = math.MaxUint16
	}

	return p
}

// TODO: use a backoff
func (c *client) connect() error {
	c.Lock()
	defer c.Unlock()
	if c.client != nil {
		return nil
	}

	// TODO: configurable timeout
	f, err := fcgi.Dial("tcp", c.addr)

	if err != nil {
		return errors.Wrap(err, "dial failed")
	}

	c.client = f

	return nil
}

func (c *client) close() {
	c.Lock()
	defer c.Unlock()

	if c.client == nil {
		return
	}
	c.client.Close()
	c.client = nil
}

func (c *client) request(r *http.Request, script string) (*fastcgiResponse, error) {
	resp := &fastcgiResponse{}

	if err := c.connect(); err != nil {
		resp.code = 502
		c.close()
		return resp, err
	}

	params := fcgi.ParamsFromRequest(r)
	params["SCRIPT_FILENAME"] = []string{script}

	response := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	req, err := c.client.BeginRequest(params, r.Body, response, stderr)
	if err != nil {
		resp.code = 500
		c.close()
		return resp, errors.Wrap(err, "failed to post to fasctgi")
	}

	if err := req.Wait(); err != nil {
		resp.code = 500
		// XXX: close the client?
		return resp, errors.Wrap(err, "failed to wait on fastcgi response")
	}

	br := bufio.NewReader(response)
	tr := textproto.NewReader(br)
	mh, err := tr.ReadMIMEHeader()
	if err != nil {
		resp.code = 500
		// XXX: close the client?
		return resp, errors.Wrap(err, "failed to read response headers")
	}
	resp.header = http.Header(mh)
	resp.code, err = statusFromHeaders(resp.header)

	if err != nil {
		resp.code = 500
		// ?? c.close()
		return resp, errors.Wrap(err, "failed to get status")
	}

	content, err := ioutil.ReadAll(br)
	if err != nil {
		resp.code = 500
		// ?? c.close()
		return resp, errors.Wrap(err, "failed to read response from fastci")
	}

	resp.body = content

	return resp, nil

}

func statusFromHeaders(h http.Header) (int, error) {
	text := h.Get("Status")

	h.Del("Status")

	if text == "" {
		return 200, nil
	}

	return strconv.Atoi(text[0:3])
}

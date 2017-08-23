package proxy

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	fcgiclient "github.com/tomasen/fcgi_client"
)

type client struct {
	*fcgiclient.FCGIClient
	pool *clientPool
}

type clientPool struct {
	addr    string
	clients chan *client
	server  *Server
}

type fastcgiResponse struct {
	code     int
	body     []byte
	response *http.Response
}

func newClientPool(s *Server, addr string, num int) *clientPool {
	p := &clientPool{
		addr:    addr,
		clients: make(chan *client, num),
		server:  s,
	}

	for i := 0; i < num; i++ {
		p.clients <- &client{pool: p}
	}
	return p
}

// XXX: should we have a maximum amount of time we will wait for a client
func (p *clientPool) acquire() *client {
	return <-p.clients
}

func (c *client) release() {
	c.pool.clients <- c
}

// TODO: use a backoff
func (c *client) connect() error {
	if c.FCGIClient != nil {
		return nil
	}

	// TODO: configurable timeout
	f, err := fcgiclient.DialTimeout("tcp", c.pool.addr, 2*time.Second)

	if err != nil {
		return errors.Wrap(err, "dial failed")
	}

	c.FCGIClient = f

	return nil
}

func (c *client) close() {
	if c.FCGIClient == nil {
		return
	}
	c.FCGIClient.Close()
	c.FCGIClient = nil
}

func (c *client) request(r *http.Request, body []byte, script string) (*fastcgiResponse, error) {
	resp := &fastcgiResponse{}

	if err := c.connect(); err != nil {
		resp.code = 502
		c.close()
		return resp, err
	}

	// TODO: more fields
	env := map[string]string{
		"SCRIPT_FILENAME":   script,
		"REQUEST_URI":       r.RequestURI,
		"REQUEST_METHOD":    r.Method,
		"CONTENT_LENGTH":    strconv.Itoa(len(body)),
		"GATEWAY_INTERFACE": "CGI/1.1",
		"QUERY_STRING":      r.URL.RawQuery,
		"DOCUMENT_ROOT":     c.pool.server.docRoot,
		"SCRIPT_NAME":       r.URL.Path,
	}

	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/grpc"
	}
	env["CONTENT_TYPE"] = ct

	rawResponse, err := c.Request(env, bytes.NewBuffer(body))
	if err != nil {
		resp.code = 500
		c.close()
		return resp, errors.Wrap(err, "failed to post to fasctgi")
	}

	defer rawResponse.Body.Close()

	content, err := ioutil.ReadAll(rawResponse.Body)
	if err != nil {
		resp.code = 500
		c.close()
		return resp, errors.Wrap(err, "failed to read response from fastci")
	}

	header := rawResponse.Header.Get("Status")
	if header != "" {
		code, err := strconv.Atoi(header[0:3])
		if err != nil {
			return nil, errors.Wrap(err, "bad status code")
		}
		rawResponse.StatusCode = code
	}

	resp.code = rawResponse.StatusCode
	resp.response = rawResponse
	resp.body = content

	return resp, nil

}

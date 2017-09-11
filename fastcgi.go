package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	fcgi "github.com/bakins/grpc-fastcgi-proxy/internal/fcgi_client"
	"github.com/pkg/errors"
)

type clientWrapper struct {
	*fcgi.FCGIClient
}

type client struct {
	sync.Mutex
	addr    string
	clients chan *clientWrapper
	server  *Server
}

type fastcgiResponse struct {
	code   int
	body   []byte
	header http.Header
}

func newClient(s *Server, addr string, num int) *client {
	p := &client{
		addr:    addr,
		server:  s,
		clients: make(chan *clientWrapper, num),
	}

	for i := 0; i < num; i++ {
		p.clients <- &clientWrapper{}
	}

	return p
}

func (c *client) acquireClient() (*clientWrapper, error) {
	w := <-c.clients

	if w.FCGIClient != nil {
		return w, nil
	}

	f, err := fcgi.DialTimeout("tcp", c.addr, 3*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "dial failed")
	}

	w.FCGIClient = f

	return w, nil
}

func (c *client) releaseClient(w *clientWrapper) {
	c.clients <- w
}

func (w *clientWrapper) close() {
	if w.FCGIClient == nil {
		return
	}

	w.FCGIClient.Close()

	w.FCGIClient = nil
}

func (c *client) request(r *http.Request, script string) (*fastcgiResponse, error) {
	resp := &fastcgiResponse{}
	w, err := c.acquireClient()
	if err != nil {
		resp.code = 500
		return resp, errors.Wrap(err, "Failed to acquire client")
	}
	defer c.releaseClient(w)

	params := map[string]string{
		"SCRIPT_FILENAME": script,
		"DOCUMENT_ROOT":   filepath.Dir(script),
		"REQUEST_METHOD":  r.Method,
		"SERVER_PROTOCOL": fmt.Sprintf("HTTP/%d.%d", r.ProtoMajor, r.ProtoMinor),
		"HTTP_HOST":       r.Host,
		"CONTENT_LENGTH":  fmt.Sprintf("%d", r.ContentLength),
		"CONTENT_TYPE":    r.Header.Get("Content-Type"),
		"REQUEST_URI":     r.RequestURI,
		"PATH_INFO":       r.URL.Path,
	}

	response, err := w.Request(params, r.Body)
	if err != nil {
		resp.code = 500
		w.close()
		return resp, errors.Wrap(err, "failed to make fasctgi request")
	}

	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		resp.code = 500
		w.close()
		return resp, errors.Wrap(err, "failed to read response from fastci")
	}

	resp.code = response.StatusCode
	resp.body = content
	resp.header = response.Header
	resp.code, err = statusFromHeaders(resp.header)

	if err != nil {
		resp.code = 500
		w.close()
		return resp, errors.Wrap(err, "failed to get status")
	}

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

package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	fcgi "github.com/bakins/grpc-fastcgi-proxy/internal/fcgi_client"
	"github.com/pkg/errors"
)

type clientWrapper struct {
	*fcgi.FCGIClient
}

type fastcgiClientPool struct {
	addr    string
	clients chan *clientWrapper
}

type fastcgiResponse struct {
	code   int
	body   []byte
	header http.Header
}

func newFastcgiClientPool(addr string, num int) *fastcgiClientPool {
	p := &fastcgiClientPool{
		addr:    addr,
		clients: make(chan *clientWrapper, num),
	}

	for i := 0; i < num; i++ {
		p.clients <- &clientWrapper{}
	}

	return p
}

func (c *fastcgiClientPool) acquireClient() (*clientWrapper, error) {
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

func (c *fastcgiClientPool) releaseClient(w *clientWrapper) {
	c.clients <- w
}

func (w *clientWrapper) close() {
	if w.FCGIClient == nil {
		return
	}

	w.FCGIClient.Close()

	w.FCGIClient = nil
}

// we acquire a client, make the request, read the full response, and release the client
// we do not want to tie up the backend connection for very long.

func (c *fastcgiClientPool) request(r *http.Request, env map[string]string) (*fastcgiResponse, error) {
	resp := &fastcgiResponse{}
	w, err := c.acquireClient()
	if err != nil {
		resp.code = 500
		return resp, errors.Wrap(err, "failed to acquire client")
	}
	defer c.releaseClient(w)

	params := map[string]string{
		"REQUEST_METHOD":    r.Method,
		"SERVER_PROTOCOL":   fmt.Sprintf("HTTP/%d.%d", r.ProtoMajor, r.ProtoMinor),
		"HTTP_HOST":         r.Host,
		"CONTENT_LENGTH":    fmt.Sprintf("%d", r.ContentLength),
		"CONTENT_TYPE":      r.Header.Get("Content-Type"),
		"REQUEST_URI":       r.RequestURI,
		"SCRIPT_NAME":       r.URL.Path,
		"GATEWAY_INTERFACE": "CGI/1.1",
		"QUERY_STRING":      r.URL.RawQuery,
	}

	for k, v := range r.Header {
		params["HTTP_"+strings.Replace(strings.ToUpper(k), "-", "_", -1)] = v[0]
	}

	for k, v := range env {
		params[k] = v
	}

	delete(params, "HTTP_PROXY")

	response, err := w.Request(params, r.Body)
	if err != nil {
		resp.code = 500
		w.close()
		return resp, errors.Wrap(err, "failed to make fastcgi request")
	}

	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		resp.code = 500
		w.close()
		return resp, errors.Wrap(err, "failed to read response from fastcgi")
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

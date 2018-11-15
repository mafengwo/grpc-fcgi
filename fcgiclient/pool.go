package fcgiclient

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

type clientWrapper struct {
	*FCGIClient
}

type FastcgiClientPool struct {
	endpoint *url.URL
	clients  chan *clientWrapper
}

func NewFastcgiClientPool(endpoint *url.URL, num int) *FastcgiClientPool {
	p := &FastcgiClientPool{
		endpoint: endpoint,
		clients:  make(chan *clientWrapper, num),
	}

	for i := 0; i < num; i++ {
		p.clients <- &clientWrapper{}
	}

	return p
}

func (c *FastcgiClientPool) acquireClient() (*clientWrapper, error) {
	w := <-c.clients

	if w.FCGIClient != nil {
		return w, nil
	}

	f, err := Dial(c.endpoint.Scheme, c.endpoint.Host,
		WithConnectTimeout(3*time.Second),
		WithKeepalive(true),
	)

	if err != nil {
		return nil, errors.Wrap(err, "dial failed")
	}

	w.FCGIClient = f

	return w, nil
}

func (c *FastcgiClientPool) releaseClient(w *clientWrapper) {
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

func (c *FastcgiClientPool) Request(p map[string]string, req io.Reader) (resp *http.Response, err error) {
	w, err := c.acquireClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire client")
	}
	defer c.releaseClient(w)

	response, err := w.Request(p, req)
	if err != nil {
		w.close()
		return nil, errors.Wrap(err, "failed to make fastcgi request")
	}
	return response, err
}

func statusFromHeaders(h http.Header) (int, error) {
	text := h.Get("Status")

	h.Del("Status")

	if text == "" {
		return 200, nil
	}

	return strconv.Atoi(text[0:3])
}

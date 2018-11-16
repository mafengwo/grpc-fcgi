package fcgiclient

import (
	"github.com/pkg/errors"
	"net/http"
	"net/url"
)

type clientWrapper struct {
	*Client
}

type ClientFactory func() *Client

type FastcgiClientPool struct {
	endpoint *url.URL

	clients       chan *clientWrapper
	size          int
	clientFactory ClientFactory
}

func NewFastcgiClientPool(size int, factory ClientFactory) *FastcgiClientPool {
	p := &FastcgiClientPool{
		size:          size,
		clientFactory: factory,
		clients:       make(chan *clientWrapper, size),
	}

	for i := 0; i < size; i++ {
		p.clients <- &clientWrapper{}
	}

	return p
}

func (c *FastcgiClientPool) acquireClient() (*clientWrapper, error) {
	w := <-c.clients

	if w.Client != nil {
		return w, nil
	}

	w.Client = c.clientFactory()

	return w, nil
}

func (c *FastcgiClientPool) releaseClient(w *clientWrapper) {
	c.clients <- w
}

func (w *clientWrapper) close() {
	if w.Client == nil {
		return
	}

	w.Client = nil
}

// we acquire a client, make the request, read the full response, and release the client
// we do not want to tie up the backend connection for very long.

func (c *FastcgiClientPool) Request(p map[string][]string, req SizedReader) ([]byte, http.Header, error) {
	w, err := c.acquireClient()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to acquire client")
	}
	defer c.releaseClient(w)

	body, header, err := w.Send(p, req)
	if err != nil {
		w.close()
		return nil, nil, errors.Wrap(err, "failed to make fastcgi request")
	}
	return body, header, err
}

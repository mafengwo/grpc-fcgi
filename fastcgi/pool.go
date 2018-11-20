package fastcgi

import (
	"github.com/pkg/errors"
	"net/http"
)

type clientWrapper struct {
	*Client
}

type ClientFactory func() *Client

type ClientPool struct {
	clients       chan *clientWrapper
	size          int
	clientFactory ClientFactory
}

func NewFastcgiClientPool(size int, factory ClientFactory) *ClientPool {
	p := &ClientPool{
		size:          size,
		clientFactory: factory,
		clients:       make(chan *clientWrapper, size),
	}

	for i := 0; i < size; i++ {
		p.clients <- &clientWrapper{}
	}

	return p
}

func (c *ClientPool) acquireClient() (*clientWrapper, error) {
	w := <-c.clients

	if w.Client != nil {
		return w, nil
	}

	cli := c.clientFactory()
	if err := cli.dialOnce(); err != nil {
		return nil, err
	}
	w.Client = cli
	return w, nil
}

func (c *ClientPool) releaseClient(w *clientWrapper) {
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

func (c *ClientPool) Send(header map[string][]string, body SizedReader) (respHeader http.Header, respBody []byte, err error) {
	w, err := c.acquireClient()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to acquire client")
	}
	defer c.releaseClient(w)

	respHeader, respBody, err = w.Send(header, body)
	if err != nil {
		w.close()
		return nil, nil, errors.Wrap(err, "failed to make fastcgi request")
	}
	return respHeader, respBody, err
}

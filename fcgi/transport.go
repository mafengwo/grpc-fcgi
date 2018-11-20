package fcgi

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"sync"
)

type connAndRequestCount struct {
	*persistConn
	usedCount    int
	maxUsedTimes int

	mu sync.Mutex
}

func (c *connAndRequestCount) hasRemains() bool {

}

func (c *connAndRequestCount) incUsedCount() {

}

func (c *connAndRequestCount) decUsedCount() {

}

type Transport struct {
	idleConnCh chan *connAndRequestCount

	MaxRequestPerConn int
	MaxConns          int

	Dial func(network, addr string) (net.Conn, error)
}

func (t *Transport) roundTrip(req *Request) (*Response, error) {
	for {
		conn, err := t.getConn(ctx)
		if err != nil {
			return nil, err
		}

		resp, err := conn.roundTrip(req)
		if err == nil {
			return resp, err
		}

		if !shouldRetryRequest(req, err) {
			return nil, err
		}
	}
}

func shouldRetryRequest(req *Request, err error) bool {
	if _, ok := err.(nothingWrittenError); ok {
		// assumed that: request body always call "rewind"
		return true
	}
	return false
}

func (t *Transport) getConn(ctx context.Context) (*connAndRequestCount, error) {
	select {
	case c := <-t.idleConnCh:
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *Transport) dialConn(ctx context.Context) (*connAndRequestCount, error) {
	conn := &persistConn{
		numExpectedResponses: 0,
		reqch:                make(chan requestAndChan, 1),
		writech:              make(chan writeRequestAndError, 1),
		closech:              make(chan struct{}),
		writeErrCh:           make(chan error, 1),
		writeLoopDone:        make(chan struct{}),
	}
	conn.br = bufio.NewReader(conn)
	conn.bw = bufio.NewWriter(conn)

	go conn.readLoop()
	go conn.writeLoop()

	return &connAndRequestCount{
		persistConn:  conn,
		usedCount:    0,
		maxUsedTimes: t.MaxRequestPerConn,
	}, nil
}

type responseAndError struct {
	res *Response
	err error
}

type requestAndChan struct {
	req        *Request
	ch         chan responseAndError
	callerGone chan<- struct{}
}

type writeRequestAndError struct {
	req *Request
	ch  chan<- error
}

// persistConn wraps a connection, usually a persistent one
type persistConn struct {
	conn net.Conn

	mu                   sync.Mutex
	numExpectedResponses int
	reqch                chan requestAndChan
	writech              chan writeRequestAndError
	closech              chan struct{}
	writeErrCh           chan error
	writeLoopDone        chan struct{}
	sawEOF               bool

	nwrite int64 // bytes written

	br *bufio.Reader // from conn
	bw *bufio.Writer // to conn
}


// only one request on-flight at most
func (pc *persistConn) roundTrip(req *Request) (*Response, error) {
	// write into write channel to be consumed by writeLoop
	writeErrCh := make(chan error, 1)
	pc.writech <- writeRequestAndError{req, writeErrCh}

	// record request-response
	resc := make(chan responseAndError)
	pc.reqch <- requestAndChan{
		req: req,
		ch:  resc,
	}

	startBytesWritten := pc.nwrite

	//TODO timer
	for {
		select {
		case err := <-writeErrCh: // write done
			if err != nil {
				return nil, err
			}
		case <-pc.closech: // connection closed
			return nil, errors.New("")
		case re := <-resc: // response received
			return re.res, re.err
		}
	}
}

func (pc *persistConn) readLoop() {
	alive := true
	for alive {
		_, err := pc.br.Peek(1)

		rc := <-pc.reqch
		var resp *Response
		if err == nil {
			resp, err = readResponse(pc.br)
		}
		rc.ch <- responseAndError{res: resp, err: err}

		alive = alive && !pc.sawEOF
	}
}

func (pc *persistConn) writeLoop() {
	for {
		select {
		case wr := <-pc.writech:
			err := wr.req.write(pc.bw)
			if err == nil {
				pc.bw.Flush()
			}
			wr.ch <- err
		case <-pc.closech: // to avoid goroutine running background on a closed conn
			return
		}
	}
}

func (pc *persistConn) Write(p []byte) (n int, err error) {
	n, err = pc.conn.Write(p)
	pc.nwrite += int64(n)
	return
}

func (pc *persistConn) Read(p []byte) (n int, err error) {
	n, err = pc.conn.Read(p)
	if err == io.EOF {
		pc.sawEOF = true
	}
	return
}

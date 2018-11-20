package fcgi

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

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
// persistConn won't close connection actively
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
	closed               bool

	nwrite int64 // bytes written

	br *bufio.Reader // from conn
	bw *bufio.Writer // to conn
}

// nothingWrittenError wraps a write errors which ended up writing zero bytes.
type nothingWrittenError struct {
	error
}

// only one request on-flight at most.
func (pc *persistConn) roundTrip(req *Request) (*Response, error) {
	fmt.Printf("\n\n")
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
	wrapError := func(err error) error {
		if startBytesWritten == pc.nwrite && err != nil {
			err = nothingWrittenError{err}
		}
		return err
	}

	//TODO timer
	for {
		select {
		case err := <-writeErrCh: // write done
			if err != nil {
				fmt.Println("write error")
				return nil, wrapError(err)
			}
		case <-pc.closech: // connection closed
			fmt.Println("connection closed")
			err := errors.New("connection closed")
			return nil, wrapError(err)
		case re := <-resc: // response received
			fmt.Println("resp received")
			res, err := re.res, re.err
			return res, wrapError(err)
		}
	}
}

func (pc *persistConn) readLoop() {
	defer func() {
		pc.close()
	}()

	for !pc.sawEOF && pc.wroteRequest() && !pc.closed {
		// peek?
		rc := <-pc.reqch

		resp, err := readResponse(pc.br)
		fmt.Printf("resp: %+v, err: %v\n", resp, err)
		rc.ch <- responseAndError{res: resp, err: err}
	}
}

func (pc *persistConn) writeLoop() {
	for {
		select {
		case wr := <-pc.writech:
			err := wr.req.write(pc.bw)
			if err == nil {
				err = pc.bw.Flush();
			}

			pc.writeErrCh <- err
			wr.ch <- err

			fmt.Printf("write done: %v\n", err)
			if err != nil {
				pc.close()
				return
			}
		case <-pc.closech: // to avoid goroutine running background on a closed conn
			fmt.Println("close signal received, stop writing")
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
	fmt.Printf("n: %d, err: %v\n", n, err)
	if err == io.EOF {
		pc.sawEOF = true
	}
	return
}

func (pc *persistConn) close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.closed {
		fmt.Println("close connection")
		close(pc.closech)
		pc.conn.Close()

		pc.closed = true
	}
}

func (pc *persistConn) wroteRequest() bool {
	//TODO timer
	err := <-pc.writeErrCh
	return err == nil
}
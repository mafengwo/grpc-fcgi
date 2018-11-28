package fcgi

import (
	"bufio"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	id   int
	t    *Transport
	conn net.Conn

	requestNumMu sync.Mutex // guard request num atomic
	requestNum   int

	mu            sync.Mutex // guard connection status
	reqch         chan requestAndChan
	writech       chan writeRequestAndError
	closech       chan struct{}
	writeErrCh    chan error
	writeLoopDone chan struct{}
	closed        bool

	nwrite int64 // bytes written

	br *bufio.Reader // from conn
	bw *bufio.Writer // to conn

	logger *zap.Logger
}

// only one request on-flight at most.
func (pc *persistConn) roundTrip(req *Request) (*Response, error) {
	pc.incRequestNum()

	pc.logDebug(zap.DebugLevel,"connection start to round trip")
	// record request-response
	resc := make(chan responseAndError)
	pc.reqch <- requestAndChan{
		req: req,
		ch:  resc,
	}

	// write into write channel to be consumed by writeLoop
	writeErrCh := make(chan error, 1)
	pc.writech <- writeRequestAndError{req, writeErrCh}

	for {
		select {
		case err := <-writeErrCh: // write done
			if err != nil {
				return nil, err
			}
		case <-pc.closech: // connection closed
			return nil, errors.New("connection closed")
		case <-req.ctx.Done():
			pc.t.putIdleConn(pc)
			return nil, errors.New("timeout")
		case re := <-resc: // response received
			if re.err == nil {
				pc.t.putIdleConn(pc)
			}
			return re.res, re.err
		}
	}
}

func (pc *persistConn) readLoop() {
	defer func() {
		pc.logDebug(zap.InfoLevel, "stop reading and start to close connection")
		pc.close()
	}()

	for !pc.closed {
		// peak first to enhance performance
		if _, err := pc.br.Peek(1); err == io.EOF {
			return
		}

		pc.logDebug(zap.DebugLevel, "waiting for response")
		resp, err := readResponse(pc.br)
		pc.logDebug(zap.DebugLevel, "read response result: %v", err)
		if err != io.EOF {
			rc := <-pc.reqch
			rc.ch <- responseAndError{res: resp, err: err} // todo in case nobody waiting
		} else if err != nil {
			return
		}
	}
}

func (pc *persistConn) writeLoop() {
	for {
		select {
		case <-pc.closech: // to avoid goroutine running background on a closed conn
			pc.logDebug(zap.InfoLevel, "close signal received, stop writing")
			return
		case wr := <-pc.writech:
			err := wr.req.write(pc.bw)
			if err == nil {
				err = pc.bw.Flush();
			}

			//pc.writeErrCh <- err
			wr.ch <- err

			pc.logDebug(zap.DebugLevel, "write result: %v", err)
			if err != nil {
				pc.close()
				return
			}
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
		pc.close()
	}
	return
}

func (pc *persistConn) close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.closed {
		// fmt.Println("close connection")
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

func (pc *persistConn) isBroken() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	return pc.closed
}

func (pc *persistConn) incRequestNum() {
	pc.requestNumMu.Lock()
	defer pc.requestNumMu.Unlock()

	pc.requestNum++
}

func (pc *persistConn) getRequestNum() int {
	pc.requestNumMu.Lock()
	defer pc.requestNumMu.Unlock()

	return pc.requestNum
}

func (pc *persistConn) logDebug(level zapcore.Level, format string, args ...interface{}) {
	if pc.logger != nil {
		pc.logger.Check(level, fmt.Sprintf(format, args...)).Write(
			zap.Int("conn_id", pc.id),
			zap.Int("request_num", pc.requestNum))
	}
}

var (
	connId      = 0
	connIdGenMu = sync.Mutex{}
)

func newConnId() int {
	connIdGenMu.Lock()
	defer connIdGenMu.Unlock()

	connId++
	return connId
}

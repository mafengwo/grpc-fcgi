package fcgi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMaxIdleConnsPerHost is the default value of Transport's
// MaxIdleConnsPerHost.
const DefaultMaxIdleConnsPerHost = 2

// connsPerHostClosedCh is a closed channel used by MaxConnsPerHost
// for the property that receives from a closed channel return the
// zero value.
var connsPerHostClosedCh = make(chan struct{})

func init() {
	close(connsPerHostClosedCh)
}

type Transport struct {
	idleMu     sync.Mutex
	idleConn   []*persistConn // most recently used at end
	idleConnCh chan *persistConn
	idleLRU    connLRU

	reqMu sync.Mutex

	altMu    sync.Mutex   // guards changing altProto only
	altProto atomic.Value // of nil or map[string]RoundTripper, key is URI scheme

	connCountMu          sync.Mutex
	connCount            int
	connPerHostAvailable chan struct{}

	Address           string
	DisableKeepAlives bool
	MaxIdleConns      int
	MaxConns          int
	IdleConnTimeout   time.Duration
	// ResponseHeaderTimeout, if non-zero, specifies the amount of
	// time to wait for a server's response headers after fully
	// writing the request (including its body, if any). This
	// time does not include the time to read the response body.
	ResponseHeaderTimeout time.Duration
	// MaxResponseHeaderBytes specifies a limit on how many
	// response bytes are allowed in the server's response
	// header.
	//
	// Zero means to use a default limit.
	MaxResponseHeaderBytes int64
}

// transportRequest is a wrapper around a *Request that adds
// optional extra headers to write and stores any error to return
// from roundTrip.
type transportRequest struct {
	*Request                     // original request, not to be mutated
	trace *httptrace.ClientTrace // optional

	mu  sync.Mutex // guards err
	err error      // first setError value for mapRoundTripError to consider
}

func (tr *transportRequest) setError(err error) {
	tr.mu.Lock()
	if tr.err == nil {
		tr.err = err
	}
	tr.mu.Unlock()
}

// roundTrip implements a RoundTripper
func (t *Transport) RoundTrip(req *Request) (*Response, bool, error) {
	ctx := req.Context()
	trace := httptrace.ContextClientTrace(ctx)

	select {
	case <-ctx.Done():
		req.closeBody()
		return nil, false, ctx.Err()
	default:
	}

	// treq gets modified by roundTrip, so we need to recreate for each retry.
	treq := &transportRequest{Request: req, trace: trace}

	pconn, err := t.getConn(treq)
	if err != nil {
		req.closeBody()
		return nil, false, err
	}

	resp, err := pconn.roundTrip(treq)
	if err == nil {
		return resp, false, nil
	}

	if !pconn.shouldRetryRequest(req, err) {
		if e, ok := err.(transportReadFromServerError); ok {
			err = e.err
		}
		return nil, false, err
	}

	return nil, true, err
}

// shouldRetryRequest reports whether we should retry sending a failed
// request on a new connection. The non-nil input error is the
// error from roundTrip.
func (pc *persistConn) shouldRetryRequest(req *Request, err error) bool {
	if !pc.isReused() {
		// This was a fresh connection. There's no reason the server
		// should've hung up on us.
		//
		// Also, if we retried now, we could loop forever
		// creating new connections and retrying if the server
		// is just hanging up on us because it doesn't like
		// our request (as opposed to sending an error).
		return false
	}

	if !req.isReplayable() {
		// Don't retry non-idempotent requests.
		return false
	}

	if _, ok := err.(transportReadFromServerError); ok {
		// We got some non-EOF net.Conn.Read failure reading
		// the 1st response byte from the server.
		return true
	}
	if err == errServerClosedIdle {
		// The server replied with io.EOF while we were trying to
		// read the response. Probably an unfortunately keep-alive
		// timeout, just as the client was writing a request.
		return true
	}
	return false // conservatively
}

// error values for debugging and testing, not seen by users.
var (
	errKeepAlivesDisabled = errors.New("putIdleConn: keep alives disabled")
	errConnBroken         = errors.New("putIdleConn: connection is in bad state")
	errTooManyIdle        = errors.New("putIdleConn: too many idle connections")
	errTooManyIdleHost    = errors.New("putIdleConn: too many idle connections for host")
	errReadLoopExiting    = errors.New("persistConn.readLoop exiting")
	errIdleConnTimeout    = errors.New("idle connection timeout")

	// errServerClosedIdle is not seen by users for idempotent requests, but may be
	// seen by a user if the server shuts down an idle connection and sends its FIN
	// in flight with already-written POST body bytes from the client.
	// See https://github.com/golang/go/issues/19943#issuecomment-355607646
	errServerClosedIdle = errors.New("http: server closed idle connection")
)

// transportReadFromServerError is used by Transport.readLoop when the
// 1 byte peek read fails and we're actually anticipating a response.
// Usually this is just due to the inherent keep-alive shut down race,
// where the server closed the connection at the same time the client
// wrote. The underlying err field is usually io.EOF or some
// ECONNRESET sort of thing which varies by platform. But it might be
// the user's custom net.Conn.Read error too, so we carry it along for
// them to return from Transport.RoundTrip.
type transportReadFromServerError struct {
	err error
}

func (e transportReadFromServerError) Error() string {
	return fmt.Sprintf("net/http: Transport failed to read from server: %v", e.err)
}

func (t *Transport) putOrCloseIdleConn(pconn *persistConn) {
	if err := t.tryPutIdleConn(pconn); err != nil {
		pconn.close(err)
	}
}

func (t *Transport) maxIdleConns() int {
	if v := t.MaxIdleConns; v != 0 {
		return v
	}
	return DefaultMaxIdleConnsPerHost
}

// tryPutIdleConn adds pconn to the list of idle persistent connections awaiting
// a new request.
// If pconn is no longer needed or not in a good state, tryPutIdleConn returns
// an error explaining why it wasn't registered.
// tryPutIdleConn does not close pconn. Use putOrCloseIdleConn instead for that.
func (t *Transport) tryPutIdleConn(pconn *persistConn) error {
	if t.DisableKeepAlives || t.MaxIdleConns < 0 {
		return errKeepAlivesDisabled
	}
	if pconn.isBroken() {
		return errConnBroken
	}

	pconn.markReused()

	t.idleMu.Lock()
	defer t.idleMu.Unlock()

	select {
	case t.idleConnCh <- pconn:
		// We're done with this pconn and somebody else is
		// currently waiting for a conn of this type (they're
		// actively dialing, but this conn is ready first).
		return nil
	default:
	}
	if t.idleConn == nil {
		t.idleConn = []*persistConn{}
	}
	idles := t.idleConn
	if len(idles) >= t.maxIdleConns() {
		return errTooManyIdleHost
	}
	for _, exist := range idles {
		if exist == pconn {
			log.Fatalf("dup idle pconn %p in freelist", pconn)
		}
	}
	t.idleConn = append(idles, pconn)
	t.idleLRU.add(pconn)
	if t.MaxIdleConns != 0 && t.idleLRU.len() > t.MaxIdleConns {
		oldest := t.idleLRU.removeOldest()
		oldest.close(errTooManyIdle)
		t.removeIdleConnLocked(oldest)
	}
	if t.IdleConnTimeout > 0 {
		if pconn.idleTimer != nil {
			pconn.idleTimer.Reset(t.IdleConnTimeout)
		} else {
			pconn.idleTimer = time.AfterFunc(t.IdleConnTimeout, pconn.closeConnIfStillIdle)
		}
	}
	pconn.idleAt = time.Now()
	return nil
}

// getIdleConnCh returns a channel to receive and return idle
func (t *Transport) getIdleConnCh() chan *persistConn {
	if t.DisableKeepAlives {
		return nil
	}
	t.idleMu.Lock()
	defer t.idleMu.Unlock()
	if t.idleConnCh == nil {
		t.idleConnCh = make(chan *persistConn)
	}
	return t.idleConnCh
}

func (t *Transport) getIdleConn() (pconn *persistConn, idleSince time.Time) {
	t.idleMu.Lock()
	defer t.idleMu.Unlock()
	for {
		if t.idleConn == nil || len(t.idleConn) == 0 {
			return nil, time.Time{}
		}

		if len(t.idleConn) == 1 {
			pconn = t.idleConn[0]
			t.idleConn = []*persistConn{}
		} else {
			// 2 or more cached connections; use the most
			// recently used one at the end.
			pconn = t.idleConn[len(t.idleConn)-1]
			t.idleConn = t.idleConn[:len(t.idleConn)-1]
		}
		t.idleLRU.remove(pconn)
		if pconn.isBroken() {
			// There is a tiny window where this is
			// possible, between the connecting dying and
			// the persistConn readLoop calling
			// Transport.removeIdleConn. Just skip it and
			// carry on.
			continue
		}
		return pconn, pconn.idleAt
	}
}

// removeIdleConn marks pconn as dead.
func (t *Transport) removeIdleConn(pconn *persistConn) {
	t.idleMu.Lock()
	defer t.idleMu.Unlock()
	t.removeIdleConnLocked(pconn)
}

// t.idleMu must be held.
func (t *Transport) removeIdleConnLocked(pconn *persistConn) {
	if pconn.idleTimer != nil {
		pconn.idleTimer.Stop()
	}
	t.idleLRU.remove(pconn)
	pconns := t.idleConn
	switch len(pconns) {
	case 0:
		// Nothing
	case 1:
		if pconns[0] == pconn {
			t.idleConn = []*persistConn{}
		}
	default:
		for i, v := range pconns {
			if v != pconn {
				continue
			}
			// Slide down, keeping most recently-used
			// conns at the end.
			copy(pconns[i:], pconns[i+1:])
			t.idleConn = pconns[:len(pconns)-1]
			break
		}
	}
}

// getConn dials and creates a new persistConn
func (t *Transport) getConn(treq *transportRequest) (*persistConn, error) {
	req := treq.Request
	trace := treq.trace
	ctx := req.Context()
	if trace != nil && trace.GetConn != nil {
		trace.GetConn(t.Address)
	}
	if pc, idleSince := t.getIdleConn(); pc != nil {
		if trace != nil && trace.GotConn != nil {
			trace.GotConn(pc.gotIdleConnTrace(idleSince))
		}
		return pc, nil
	}

	type dialRes struct {
		pc  *persistConn
		err error
	}
	dialc := make(chan dialRes)

	// Copy these hooks so we don't race on the postPendingDial in
	// the goroutine we launch. Issue 11136.
	testHookPrePendingDial := testHookPrePendingDial
	testHookPostPendingDial := testHookPostPendingDial

	handlePendingDial := func() {
		testHookPrePendingDial()
		go func() {
			if v := <-dialc; v.err == nil {
				t.putOrCloseIdleConn(v.pc)
			} else {
				t.decHostConnCount()
			}
			testHookPostPendingDial()
		}()
	}

	if t.MaxConns > 0 {
		select {
		case <-t.incHostConnCount():
			// count below conn per host limit; proceed
		case pc := <-t.getIdleConnCh():
			if trace != nil && trace.GotConn != nil {
				trace.GotConn(httptrace.GotConnInfo{Conn: pc.conn, Reused: pc.isReused()})
			}
			return pc, nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	go func() {
		pc, err := t.dialConn(ctx)
		dialc <- dialRes{pc, err}
	}()

	idleConnCh := t.getIdleConnCh()
	select {
	case v := <-dialc:
		// Our dial finished.
		if v.pc != nil {
			if trace != nil && trace.GotConn != nil {
				trace.GotConn(httptrace.GotConnInfo{Conn: v.pc.conn})
			}
			return v.pc, nil
		}
		// Our dial failed. See why to return a nicer error
		// value.
		t.decHostConnCount()
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		default:
			// It wasn't an error due to cancelation, so
			// return the original error message:
			return nil, v.err
		}
	case pc := <-idleConnCh:
		// Another request finished first and its net.Conn
		// became available before our dial. Or somebody
		// else's dial that they didn't use.
		// But our dial is still going, so give it away
		// when it finishes:
		handlePendingDial()
		if trace != nil && trace.GotConn != nil {
			trace.GotConn(httptrace.GotConnInfo{Conn: pc.conn, Reused: pc.isReused()})
		}
		return pc, nil
	case <-req.Context().Done():
		handlePendingDial()
		return nil, req.Context().Err()
	}
}

// incHostConnCount increments the count of connections
func (t *Transport) incHostConnCount() <-chan struct{} {
	if t.MaxConns <= 0 {
		return connsPerHostClosedCh
	}

	t.connCountMu.Lock()
	defer t.connCountMu.Unlock()
	if t.connCount == t.MaxConns {
		if t.connPerHostAvailable == nil {
			t.connPerHostAvailable = make(chan struct{})
		}
		return t.connPerHostAvailable
	}

	t.connCount++
	return connsPerHostClosedCh
}

// decHostConnCount decrements the count of connections
func (t *Transport) decHostConnCount() {
	if t.MaxConns <= 0 {
		return
	}

	t.connCountMu.Lock()
	defer t.connCountMu.Unlock()

	t.connCount--

	select {
	case t.connPerHostAvailable <- struct{}{}:
	default:
		// close channel before deleting avoids getConn waiting forever in
		// case getConn has reference to channel but hasn't started waiting.
		/*
		if t.connPerHostAvailable != nil {
			close(t.connPerHostAvailable)
		}
		*/
	}
}

// connCloseListener wraps a connection, the transport that dialed it
// and the connected-to host key so the host connection count can be
// transparently decremented by whatever closes the embedded connection.
type connCloseListener struct {
	net.Conn
	t        *Transport
	didClose int32
}

func (c *connCloseListener) Close() error {
	if atomic.AddInt32(&c.didClose, 1) != 1 {
		return nil
	}
	err := c.Conn.Close()
	c.t.decHostConnCount()
	return err
}

func (t *Transport) dialConn(ctx context.Context) (*persistConn, error) {
	pconn := &persistConn{
		t:             t,
		reqch:         make(chan requestAndChan, 1),
		writech:       make(chan writeRequest, 1),
		closech:       make(chan struct{}),
		writeErrCh:    make(chan error, 1),
		writeLoopDone: make(chan struct{}),
	}

	var defaultDialer net.Dialer
	conn, err := defaultDialer.DialContext(ctx, "tcp", t.Address)
	if err != nil {
		return nil, err
	}
	pconn.conn = conn

	if t.MaxConns > 0 {
		pconn.conn = &connCloseListener{Conn: pconn.conn, t: t}
	}
	pconn.br = bufio.NewReader(pconn)
	pconn.bw = bufio.NewWriter(persistConnWriter{pconn})
	go pconn.readLoop()
	go pconn.writeLoop()
	return pconn, nil
}

package fcgi

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
)

type Transport struct {
	idleConnCh chan *persistConn
	idleMu     sync.Mutex
	idleConn   []*persistConn

	MaxConn int

	Address string
	Dial    func(network, addr string) (net.Conn, error)

	connCountMu   sync.Mutex
	connCount     int
	connAvailable chan struct{}
}

func (t *Transport) RoundTrip(req *Request) (*Response, error) {
	fmt.Printf("\n\n\n")
	fmt.Println("transport round trip")
	for {
		//TODO context.Done()
		conn, err := t.getConn(nil)
		if err != nil {
			return nil, err
		}

		resp, err := conn.roundTrip(req)
		fmt.Printf("connection round trip return resp: %v, error: %v", resp != nil, err)
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

func (t *Transport) getConn(ctx context.Context) (*persistConn, error) {
	fmt.Println("try to get idle conn")
	// get idle connection
	if pc := t.getIdleConn(); pc != nil {
		fmt.Println("got idle conn")
		return pc, nil
	}

	if t.MaxConn > 0 {
		select {
		case <-t.incConnCount():
			fmt.Println("count below limit")
			// count below conn limit; proceed
		case pc := <-t.getIdleConnCh():
			return pc, nil
		}
	}

	// dial new connection if no idle connection available
	type dialRes struct {
		pc  *persistConn
		err error
	}
	dialc := make(chan dialRes)
	go func() {
		pc, err := t.dialConn(ctx)
		dialc <- dialRes{pc, err}
	}()

	handlePendingDial := func() {
		go func() {
			if v := <-dialc; v.err == nil {
				t.putIdleConn(v.pc)
			} else {
				t.decConnCount()
			}
		}()
	}

	select {
	case ds := <-dialc:
		fmt.Println("dialed")
		if ds.err != nil {
			// Once dial failed. decrement the connection count
			t.decConnCount()
		}
		return ds.pc, ds.err
	case pc := <-t.getIdleConnCh():
		// Another request finished first and its net.Conn
		// became available before our dial. Or somebody
		// else's dial that they didn't use.
		// But our dial is still going, so give it away
		// when it finishes:
		fmt.Println("got idle conn ch")
		handlePendingDial()
		return pc, nil
	}
}

func (t *Transport) getIdleConnCh() chan *persistConn {
	t.idleMu.Lock()
	defer t.idleMu.Unlock()

	if t.idleConnCh == nil {
		t.idleConnCh = make(chan *persistConn)
	}

	return t.idleConnCh
}

func (t *Transport) getIdleConn() *persistConn {
	t.idleMu.Lock()
	defer t.idleMu.Unlock()

	for {
		if t.idleConn == nil || len(t.idleConn) == 0 {
			return nil
		}

		pc := t.idleConn[len(t.idleConn)-1]
		t.idleConn = t.idleConn[:len(t.idleConn)-1]
		if !pc.isBroken() {
			return pc
		}
	}
}

// putIdleConn put conn into idleConn OR idleConnCh
func (t *Transport) putIdleConn(pc *persistConn) {
	t.idleMu.Lock()
	defer t.idleMu.Unlock()

	if pc.isBroken() {
		return
	}

	select {
	case t.idleConnCh <- pc:
		// Done here means somebody is currently waiting,
		// conn cannot put in idleConn in this situation
		return
	default:
	}

	if t.idleConn == nil {
		t.idleConn = []*persistConn{}
	}
	t.idleConn = append(t.idleConn, pc)
}

func (t *Transport) dialConn(ctx context.Context) (*persistConn, error) {
	netconn, err := t.Dial("tcp", t.Address)
	if err != nil {
		return nil, err
	}

	conn := &persistConn{
		t:             t,
		conn:          netconn,
		reqch:         make(chan requestAndChan, 1),
		writech:       make(chan writeRequestAndError, 1),
		closech:       make(chan struct{}),
		writeErrCh:    make(chan error, 1),
		writeLoopDone: make(chan struct{}),
	}
	conn.br = bufio.NewReader(conn)
	conn.bw = bufio.NewWriter(conn)

	go conn.readLoop()
	go conn.writeLoop()
	go func() {
		<-conn.closech
		t.decConnCount()
	}()

	return conn, nil
}

var closedCh = make(chan struct{})

// incConnCount increments the count of connections.
// It returns an already-closed channel if the count
// is not at its limit; otherwise it returns a channel which is
// notified when the count is below the limit.
func (t *Transport) incConnCount() <-chan struct{} {
	if t.MaxConn <= 0 {
		return closedCh
	}

	t.connCountMu.Lock()
	defer t.connCountMu.Unlock()

	if t.connCount == t.MaxConn {
		if t.connAvailable == nil {
			t.connAvailable = make(chan struct{})
		}
		return t.connAvailable
	}

	t.connCount++

	return closedCh
}

func (t *Transport) decConnCount() {
	if t.MaxConn <= 0 {
		return
	}

	t.connCountMu.Lock()
	defer t.connCountMu.Unlock()

	t.connCount--

	select {
	case t.connAvailable <- struct{}{}:
	default:
		// close channel before deleting avoids getConn waiting forever in
		// case getConn has reference to channel but hasn't started waiting.
		// This could lead to more than MaxConnsPerHost in the unlikely case
		// that > 1 go routine has fetched the channel but none started waiting.
		/*
		if t.connAvailable != nil {
			close(t.connAvailable)
		}
		*/
	}
}

func init() {
	close(closedCh)
}

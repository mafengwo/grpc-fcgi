package fcgi

import (
	"bufio"
	"context"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"sync"
)

type Transport struct {
	idleConnCh chan *persistConn
	idleMu     sync.Mutex
	idleConn   []*persistConn

	MaxConn int

	Address              string
	Dial                 func(network, addr string) (net.Conn, error)
	ConnectionMaxRequest int
	Logger               *zap.Logger

	connCountMu   sync.Mutex
	connCount     int
	connAvailable chan struct{}
}

func (t *Transport) RoundTrip(req *Request) (*Response, error) {
	for {
		//TODO context.Done()
		conn, err := t.getConn(req, nil)
		if err != nil {
			return nil, err
		}

		startBytesWritten := conn.nwrite

		resp, err := conn.roundTrip(req)
		if err == nil {
			return resp, err
		}

		if conn.nwrite != startBytesWritten {
			return nil, err
		}

		t.logRequest(req, zap.InfoLevel, "retrying...")
	}
}

func (t *Transport) getConn(req *Request, ctx context.Context) (*persistConn, error) {
	// get idle connection
	if pc := t.getIdleConn(); pc != nil {
		t.logRequest(req, zap.DebugLevel, "got idle conn: %d", pc.id)
		return pc, nil
	}

	t.logRequest(req, zap.DebugLevel, "no idle connection for now")
	if t.MaxConn > 0 {
		select {
		case <-t.incConnCount(): // count below conn limit; proceed
			t.logRequest(req, zap.DebugLevel, "current connection still under MaxConn limit")
		case pc := <-t.getIdleConnCh():
			t.logRequest(req, zap.DebugLevel, "over MaxConn limit, but someone released %d", pc.id)
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
		if ds.err != nil {
			// Once dial failed. decrement the connection count
			t.decConnCount()
			t.logRequest(req, zap.ErrorLevel, "dial failed: %v", ds.err)
		} else {
			t.logRequest(req, zap.DebugLevel, "dial success, connection id: %d", ds.pc.id)
		}
		return ds.pc, ds.err
	case pc := <-t.getIdleConnCh():
		// Another request finished first and its net.Conn
		// became available before our dial. Or somebody
		// else's dial that they didn't use.
		// But our dial is still going, so give it away
		// when it finishes:
		t.logRequest(req, zap.DebugLevel, "connection: %d released", pc.id)
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

		pc := t.idleConn[0] // get the LRU connection
		t.idleConn = t.idleConn[1:]
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
		//t.logRequest("connection %d be willing to put into idle list, but aborted for broken", pc.id)
		return
	}

	// replace the staled connection with a new one
	// and then put into idle list
	if t.ConnectionMaxRequest > 0 && pc.getRequestNum() >= t.ConnectionMaxRequest {
		go func() {
			pc.close() // avoid goroutine leaks

			conn, err := t.dialConn(nil)
			if err != nil {
				return
			}
			t.putIdleConn(conn)
		}()
		return
	}

	select {
	case t.idleConnCh <- pc:
		// Done here means somebody is currently waiting,
		// conn cannot put in idleConn in this situation
		// t.logRequest("connection %d been taken by waiting task before put into idle list", pc.id)
		return
	default:
	}

	// t.logRequest("connection %d been put into idle list", pc.id)
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
		id:            newConnId(),
		t:             t,
		conn:          netconn,
		reqch:         make(chan requestAndChan, 1),
		writech:       make(chan writeRequestAndError, 1),
		closech:       make(chan struct{}),
		writeErrCh:    make(chan error, 1),
		writeLoopDone: make(chan struct{}),
	}
	if t.Logger != nil {
		conn.logger = t.Logger.With()
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

func (t *Transport) logRequest(req *Request, level zapcore.Level, format string, args ...interface{}) {
	if t.Logger != nil {
		msg := fmt.Sprintf(format, args...)

		fields := []zap.Field{
			zap.String("layer", "transport"),
		}
		if requestId, yes := req.ctx.Value("id").(string); yes {
			fields = append(fields, zap.String("request_id", requestId))
		}
		t.Logger.Check(level, msg).Write(fields...)
	}
}

func init() {
	close(closedCh)
}

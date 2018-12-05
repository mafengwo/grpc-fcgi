package fcgi

import (
	"bytes"
	"context"
	"io"
	"net/http/httptrace"
	"strconv"
	"sync"
	"time"
)

type TraceInfo struct {
	ConnectStartTime         time.Time
	ConnectDoneTime          time.Time
	GetConnTime              time.Time
	GotConnTime              time.Time
	WroteHeadersTime         time.Time
	WroteRequestTime         time.Time
	GotFirstResponseByteTime time.Time
	PutIdleTime              time.Time
	PutIdleError             error
}

type Request struct {
	Header map[string][]string
	Body   []byte

	Trace *TraceInfo

	Ctx   context.Context
	ctxMu sync.Mutex
}

func (r *Request) write(w io.Writer) (err error) {
	trace := httptrace.ContextClientTrace(r.Context())
	if trace != nil && trace.WroteRequest != nil {
		defer func() {
			trace.WroteRequest(httptrace.WroteRequestInfo{
				Err: err,
			})
		}()
	}

	var buf buffer
	buf.Reset()

	if err := writeBeginReq(w, &buf, requestID); err != nil {
		return err
	}

	// if CONTENT_LENGTH is missed or mismatch the body length, the all following
	// request will be ruined. so CONTENT_LENGTH must be override
	r.Header["CONTENT_LENGTH"] = []string{strconv.Itoa(len(r.Body))}
	if err := writeParams(w, &buf, requestID, r.Header); err != nil {
		return err
	}
	if trace != nil && trace.WroteHeaders != nil {
		trace.WroteHeaders()
	}

	var bodyBuf buffer
	bodyBuf.Reset()
	return writeStdin(w, &bodyBuf, requestID, bytes.NewReader(r.Body))
}

func (r *Request) WithTrace() *Request {
	r.initOnce()

	trace := &httptrace.ClientTrace{
		GetConn: func(hostPort string) { r.Trace.GetConnTime = time.Now() },
		GotConn: func(conn httptrace.GotConnInfo) { r.Trace.GotConnTime = time.Now() },
		PutIdleConn: func(err error) {
			r.Trace.PutIdleTime = time.Now()
			if err != nil {
				r.Trace.PutIdleError = err
			}
		},
		GotFirstResponseByte: func() { r.Trace.GotFirstResponseByteTime = time.Now() },
		ConnectStart:         func(network, addr string) { r.Trace.ConnectStartTime = time.Now() },
		ConnectDone:          func(network, addr string, err error) { r.Trace.ConnectDoneTime = time.Now() },
		WroteHeaders:         func() { r.Trace.WroteHeadersTime = time.Now() },
		WroteRequest:         func(reqInfo httptrace.WroteRequestInfo) { r.Trace.WroteRequestTime = time.Now() },
	}
	r.Ctx = httptrace.WithClientTrace(r.Ctx, trace)
	return r
}

func (r *Request) Context() context.Context {
	return r.Ctx
}

type requestIdKey struct{}

func (r *Request) WithRequestId(id string) *Request {
	r.initOnce()

	r.Ctx = context.WithValue(r.Ctx, requestIdKey{}, id)
	return r
}

func (r *Request) GetRequestId() (string, bool) {
	r.initOnce()

	v, ok := r.Ctx.Value(requestIdKey{}).(string)
	return v, ok
}

func (r *Request) isReplayable() bool {
	return true
}

func (r *Request) closeBody() {

}

func (r *Request) initOnce() {
	r.ctxMu.Lock()
	defer r.ctxMu.Unlock()

	if r.Ctx == nil {
		r.Ctx = context.Background()
	}

	if r.Trace == nil {
		r.Trace = &TraceInfo{}
	}
}

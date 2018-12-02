package transport

import (
	"bytes"
	"context"
	"io"
	"net/http/httptrace"
	"strconv"
)

type SizedReader interface {
	io.Reader
	Size() int64
}

type Request struct {
	Header map[string][]string
	Body   []byte

	ctx context.Context
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
	if trace != nil && trace.WroteHeaderField != nil {
		trace.WroteHeaders()
	}

	var bodyBuf buffer
	bodyBuf.Reset()
	return writeStdin(w, &bodyBuf, requestID, bytes.NewReader(r.Body))
}

func (r *Request) Context() context.Context {
	return r.ctx
}

func (r *Request) WithContext(ctx context.Context) *Request {
	r.ctx = ctx
	return r
}

type requestIdKey struct{}

func (r *Request) WithRequestId(id string) *Request {
	if r.ctx != nil {
		r.ctx = context.WithValue(r.ctx, requestIdKey{}, id)
	}
	return r
}

func (r *Request) GetRequestId() (string, bool) {
	if r.ctx != nil {
		v, ok := r.ctx.Value(requestIdKey{}).(string)
		return v, ok
	}
	return "", false
}

func (r *Request) isReplayable() bool {
	return true
}

func (r *Request) closeBody() {

}

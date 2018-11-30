package transport

import (
	"bufio"
	"context"
	"io"
	"strconv"
)

type SizedReader interface {
	io.Reader
	Size() int64
}

type Request struct {
	Header map[string][]string
	Body SizedReader

	ctx context.Context
}

func (r *Request) write(w io.Writer) error {
	var buf buffer
	buf.Reset()

	if err := writeBeginReq(w, &buf, requestID); err != nil {
		return err
	}

	// if CONTENT_LENGTH is missed or mismatch the body length, the all following
	// request will be ruined. so CONTENT_LENGTH must be override
	r.Header["CONTENT_LENGTH"] = []string{strconv.Itoa(int(r.Body.Size()))}
	if err := writeParams(w, &buf, requestID, r.Header); err != nil {
		return err
	}

	var bodyBuf buffer
	bodyBuf.Reset()
	return writeStdin(w, &bodyBuf, requestID, bufio.NewReader(r.Body))
}

func (r *Request) WithContext(ctx context.Context) (*Request) {
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


package fcgi

import (
	"bufio"
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


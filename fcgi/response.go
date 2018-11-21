package fcgi

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http/httputil"
	"net/textproto"
)

type Response struct {
	Headers      map[string][]string
	Body         []byte
	ErrorMessage []byte
}

func readResponse(c io.Reader) (*Response, error) {
	var h header
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	for {
		if err := binary.Read(c, binary.BigEndian, &h); err != nil {
			return nil, err
		}

		buf := make([]byte, int(h.ContentLength)+int(h.PaddingLength))
		if _, err := io.ReadFull(c, buf); err != nil {
			return nil, err
		}

		buf = buf[:h.ContentLength]
		switch h.Type {
		case typeStdout:
			if _, err := stdout.Write(buf); err != nil {
				return nil, err
			}
		case typeStderr:
			if _, err := stderr.Write(buf); err != nil {
				return nil, err
			}
		case typeEndRequest:
			goto PARSE
		default:
			return nil, fmt.Errorf("unexpected type: %d", h.Type)
		}
	}
PARSE:
	rb := bufio.NewReader(stdout)
	tp := textproto.NewReader(rb)

	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		err = errors.WithMessage(err, "failed to parse FCGI response header")
		return nil, err
	}

	var body []byte
	te := mimeHeader["Transfer-Encoding"]
	if len(te) > 0 && te[0] == "chunked" {
		body, err = ioutil.ReadAll(httputil.NewChunkedReader(rb))
		if err != nil {
			err = errors.WithMessage(err, "failed to parse FCGI chunked response")
			return nil, err
		}
	} else {
		body, err = ioutil.ReadAll(rb)
		if err != nil {
			err = errors.WithMessage(err, "failed to parse FCGI response")
			return nil, err
		}
	}

	errmsg, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get stderr")
	}

	return &Response{
		Headers:      mimeHeader,
		Body:         body,
		ErrorMessage: errmsg,
	}, nil
}

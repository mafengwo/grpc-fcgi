package transport

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

type body struct {
	src          io.Reader
	hdr          interface{}   // non-nil (Response or Request) value means read trailer
	r            *bufio.Reader // underlying wire-format reader for the trailer
	closing      bool          // is the connection to be closed after reading body?
	doEarlyClose bool          // whether Close should stop early

	mu         sync.Mutex // guards following, and calls to Read and Close
	sawEOF     bool
	closed     bool
	earlyClose bool   // Close called and we didn't read to the end of src
	onHitEOF   func() // if non-nil, func to call when EOF is Read
}

var ErrBodyReadAfterClose = errors.New("http: invalid Read on closed Body")

func (b *body) Read(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0, ErrBodyReadAfterClose
	}
	return b.readLocked(p)
}

// Must hold b.mu.
func (b *body) readLocked(p []byte) (n int, err error) {
	if b.sawEOF {
		return 0, io.EOF
	}
	n, err = b.src.Read(p)

	if err == io.EOF {
		b.sawEOF = true
		if lr, ok := b.src.(*io.LimitedReader); ok && lr.N > 0 {
			err = io.ErrUnexpectedEOF
		}
	}

	// If we can return an EOF here along with the read data, do
	// so. This is optional per the io.Reader contract, but doing
	// so helps the HTTP transport code recycle its connection
	// earlier (since it will see this EOF itself), even if the
	// client doesn't do future reads or Close.
	if err == nil && n > 0 {
		if lr, ok := b.src.(*io.LimitedReader); ok && lr.N == 0 {
			err = io.EOF
			b.sawEOF = true
		}
	}

	if b.sawEOF && b.onHitEOF != nil {
		b.onHitEOF()
	}

	return n, err
}

// maxPostHandlerReadBytes is the max number of Request.Body bytes not
// consumed by a handler that the server will read from the client
// in order to keep a connection alive. If there are more bytes than
// this then the server to be paranoid instead sends a "Connection:
// close" response.
//
// This number is approximately what a typical machine's TCP buffer
// size is anyway.  (if we have the bytes on the machine, we might as
// well read them)
const maxPostHandlerReadBytes = 256 << 10

func (b *body) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	var err error
	switch {
	case b.sawEOF:
		// Already saw EOF, so no need going to look for it.
	case b.hdr == nil && b.closing:
		// no trailer and closing the connection next.
		// no point in reading to EOF.
	case b.doEarlyClose:
		// Read up to maxPostHandlerReadBytes bytes of the body, looking for
		// for EOF (and trailers), so we can re-use this connection.
		if lr, ok := b.src.(*io.LimitedReader); ok && lr.N > maxPostHandlerReadBytes {
			// There was a declared Content-Length, and we have more bytes remaining
			// than our maxPostHandlerReadBytes tolerance. So, give up.
			b.earlyClose = true
		} else {
			var n int64
			// Consume the body, or, which will also lead to us reading
			// the trailer headers after the body, if present.
			n, err = io.CopyN(ioutil.Discard, bodyLocked{b}, maxPostHandlerReadBytes)
			if err == io.EOF {
				err = nil
			}
			if n == maxPostHandlerReadBytes {
				b.earlyClose = true
			}
		}
	default:
		// Fully consume the body, which will also lead to us reading
		// the trailer headers after the body, if present.
		_, err = io.Copy(ioutil.Discard, bodyLocked{b})
	}
	b.closed = true
	return err
}

func bodyReadBuff(r *bufio.Reader, len int64) *body {
	return &body{src: io.LimitReader(r, len), closing: false}
}

// bodyLocked is a io.Reader reading from a *body when its mutex is
// already held.
type bodyLocked struct {
	b *body
}

func (bl bodyLocked) Read(p []byte) (n int, err error) {
	if bl.b.closed {
		return 0, ErrBodyReadAfterClose
	}
	return bl.b.readLocked(p)
}

type Response struct {
	Headers      map[string][]string
	Request      *Request
	Body         io.ReadCloser
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

	te := mimeHeader["Transfer-Encoding"]
	if len(te) > 0 && te[0] == "chunked" {
		err = errors.WithMessage(err, "failed to parse FCGI chunked response")
		return nil, err
	}

	bl, err := parseBodyLength(mimeHeader)
	if err != nil {
		return nil, err
	}

	return &Response{
		Headers:      mimeHeader,
		Body:         bodyReadBuff(rb, bl),
	}, nil
}

func (r *Response) GetStatusCode() (int, error) {
	vals, exist := r.Headers["Status"]
	if !exist || len(vals) == 0 || vals[0] == "" {
		return 200, nil
	}

	return strconv.Atoi(vals[0][0:3])
}

func parseBodyLength(header textproto.MIMEHeader) (int64, error) {
	contentLens := header["Content-Length"]
	var cl string
	if len(contentLens) == 1 {
		cl = strings.TrimSpace(contentLens[0])
	}

	if cl != "" {
		cl = strings.TrimSpace(cl)
		if cl == "" {
			return -1, errors.New("no content-type found in header")
		}
		n, err := strconv.ParseInt(cl, 10, 64)
		if err != nil || n < 0 {
			return 0, errors.New("bad content-length")
		}
		return n, nil
	}

	return -1, errors.New("unexpected content-type in header")
}
package fcgi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
)

// Request ...
type Request struct {
	c      *conn
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

func writerOrDiscard(w io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return ioutil.Discard
}

// StdoutPipe ...
func (r *Request) StdoutPipe() io.Reader {
	pr, pw := io.Pipe()
	r.Stdout = pw
	return pr
}

// StderrPipe ...
func (r *Request) StderrPipe() io.Reader {
	pr, pw := io.Pipe()
	r.Stderr = pw
	return pr
}

// Wait ...
func (r *Request) Wait() error {
	defer r.c.Close()

	var h header

	stdout := writerOrDiscard(r.Stdout)
	stderr := writerOrDiscard(r.Stderr)

	go func() {
		var buf buffer
		buf.Reset()

		if err := writeStdin(r.c, &buf, requestID, r.Stdin); err != nil {
			// TODO(knorton): propagate the correct error as the return value.
			r.c.Close()
		}
	}()

	for {
		if err := binary.Read(r.c, binary.BigEndian, &h); err != nil {
			return err
		}

		if h.Version != fcgiVersion {
			return errors.New("invalid fcgi version")
		}

		if h.ID != 1 {
			return errors.New("invalid request id")
		}

		buf := make([]byte, int(h.ContentLength)+int(h.PaddingLength))
		if _, err := io.ReadFull(r.c, buf); err != nil {
			return err
		}

		buf = buf[:h.ContentLength]
		switch h.Type {
		case typeStdout:
			if _, err := stdout.Write(buf); err != nil {
				return err
			}
		case typeStderr:
			if _, err := stderr.Write(buf); err != nil {
				return err
			}
		case typeEndRequest:
			return nil
		default:
			return fmt.Errorf("unexpected type: %d", h.Type)
		}
	}
}

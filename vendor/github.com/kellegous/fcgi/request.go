package fcgi

import "io"

type stdout []byte
type stderr []byte

// Request is an in-flight request to the FCGI server.
type Request struct {
	id       uint16
	c        *Client
	cw       chan interface{}
	done     chan struct{}
	out, err io.Writer
}

// Abort the request by issuing an FCGI_ABORT_REQUEST abort request
// to the server.
func (r *Request) Abort() error {
	var buf buffer
	return writeAbortReq(r.c.c, &buf, r.id)
}

// ID is the identifier for this request that is used as a part of the FCGI
// protocol.
func (r *Request) ID() uint16 {
	return r.id
}

// Wait for this process to be processed by the FCGI server.
func (r *Request) Wait() error {
	for item := range r.cw {
		switch t := item.(type) {
		case stdout:
			if len(t) == 0 {
				continue
			}
			if _, err := r.out.Write([]byte(t)); err != nil {
				r.drain()
				return err
			}
		case stderr:
			if len(t) == 0 {
				continue
			}
			if _, err := r.err.Write([]byte(t)); err != nil {
				r.drain()
				return err
			}
		case error:
			return t
		default:
			// t will always be nil
			return nil
		}
	}
	return nil
}

// drain detaches the request from the client and discards any pending
// messages that are waiting to be processed.
func (r *Request) drain() {
	r.c.unsub(r.id)
	select {
	case <-r.cw:
	default:
	}
}

package transport

import (
	"encoding/binary"
	"io"
	"net"
)

type recType uint8

const (
	typeBeginRequest recType = iota + 1
	typeAbortRequest
	typeEndRequest
	typeParams
	typeStdin
	typeStdout
	typeStderr
	typeData
	typeGetValues
	typeGetValuesResult
	typeUnknownType
)

type header struct {
	Version       uint8
	Type          recType
	ID            uint16
	ContentLength uint16
	PaddingLength uint8
	Reserved      uint8
}

const (
	maxWrite            = 65535
	maxPad              = 0
	fcgiVersion  uint8  = 1
	flagNone     uint8  = 0
	flagKeepConn uint8  = 1
	requestID    uint16 = 1
)

const (
	roleResponder uint16 = iota + 1
	roleAuthorizer
	roleFilter
)

const (
	statusRequestComplete = iota
	statusCantMultiplex
	statusOverloaded
	statusUnknownRole
)

// Write the beginning of a request into the given connection.
func writeBeginReq(c io.Writer, w *buffer, id uint16) error {
	binary.Write(w, binary.BigEndian, roleResponder) // role
	binary.Write(w, binary.BigEndian, flagKeepConn)  // flags
	w.Write([]byte{0, 0, 0, 0, 0})                   // reserved
	return w.WriteRecord(c, id, typeBeginRequest)
}

// Write an abort request into the given connection.
func writeAbortReq(c net.Conn, w *buffer, id uint16) error {
	return w.WriteRecord(c, id, typeAbortRequest)
}

// Encode the length of a key or value using FCGIs compressed length
// scheme. The encoded length is placed in b and the number of bytes
// that were required to encode the length is returned.
func encodeLength(b []byte, n uint32) int {
	if n > 127 {
		n |= 1 << 31
		binary.BigEndian.PutUint32(b, n)
		return 4
	}
	b[0] = byte(n)
	return 1
}

// Encode and write the given parameters into the connection. Note that the headers
// may be fragmented into several writes if they will not fit into a single write.
func writeParams(c io.Writer, w *buffer, id uint16, params map[string][]string) error {
	var b [8]byte
	for key, vals := range params {
		for _, val := range vals {
			// encode the key's length
			n := encodeLength(b[:], uint32(len(key)))

			// encode the value's length
			n += encodeLength(b[n:], uint32(len(val)))

			// the total lenth of this param
			t := n + len(key) + len(val)

			// this header itself is so big, it cannot fit into a
			// write so we just discard it.
			if t > w.Cap() {
				continue
			}

			// if this param would overflow the current buffer, go ahead
			// and send it.
			if t > w.Free() {
				if err := w.WriteRecord(c, id, typeParams); err != nil {
					return err
				}
			}

			w.Write(b[:n])
			w.Write([]byte(key))
			w.Write([]byte(val))
		}
	}

	if w.Len() > 0 {
		if err := w.WriteRecord(c, id, typeParams); err != nil {
			return err
		}
	}

	// send the empty params message
	return w.WriteRecord(c, id, typeParams)
}

// Copy the data from the given reader into the connection as stdin. Note that
// this may fragment the data into multiple writes.
func writeStdin(c io.Writer, w *buffer, id uint16, r io.Reader) error {
	if r != nil {
		for {
			err := w.CopyFrom(r)
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}

			if err := w.WriteRecord(c, id, typeStdin); err != nil {
				return err
			}
		}
	}

	return w.WriteRecord(c, id, typeStdin)
}

package fcgi

import (
	"encoding/binary"
	"io"
)

// A buffer that holds an outgoing FCGI record. Note that a newly
// created buffer must be initialized with a call to Reset before
// it can be used.
type buffer struct {
	ix int
	dt [maxWrite + maxPad + 8]byte
}

// Update the header of the buffer.
func (b *buffer) WriteHeader(id uint16, recType recType, n int) {
	b.dt[0] = byte(fcgiVersion)                      // the fcgi version
	b.dt[1] = byte(recType)                          // the record type
	binary.BigEndian.PutUint16(b.dt[2:4], id)        // the request id
	binary.BigEndian.PutUint16(b.dt[4:6], uint16(n)) // the size of the record
	b.dt[6] = 0                                      // reserved
	b.dt[7] = 0                                      // reserved
}

// Write the data into the buffer.
func (b *buffer) Write(p []byte) (int, error) {
	n := len(p)
	copy(b.dt[b.ix:], p)
	b.ix += n
	return n, nil
}

// Copy the bytes from the Reader into the buffer. This method will
// never copy more than the maxWrite size of the buffer.
func (b *buffer) CopyFrom(r io.Reader) error {
	n, err := r.Read(b.dt[b.ix:])
	if err != nil {
		return err
	}
	b.ix += n
	return nil
}

// Clear the contents of the buffer.
func (b *buffer) Reset() {
	// the first 8 bytes are reserved for the header.
	b.ix = 8
}

// Get the bytes of the buffer back as a slice.
func (b *buffer) Bytes() []byte {
	return b.dt[:b.ix]
}

// Get the total possible capacity of the buffer.
func (b *buffer) Cap() int {
	return len(b.dt) - 8
}

// Get the current length of the buffer.
func (b *buffer) Len() int {
	return b.ix - 8
}

// Get the number of free bytes left in the buffer.
func (b *buffer) Free() int {
	return len(b.dt) - b.ix
}

// Write the contents of the buffer as a record of the given type for the given
// request id.
func (b *buffer) WriteRecord(w io.Writer, id uint16, recType recType) error {
	b.WriteHeader(id, recType, b.Len())
	_, err := w.Write(b.Bytes())
	b.Reset()
	return err
}

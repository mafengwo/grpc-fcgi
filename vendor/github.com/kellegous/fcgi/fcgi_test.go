package fcgi

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	std "net/http/fcgi"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
)

type mockServer struct {
	list net.Listener
	dir  string
	t    *testing.T
}

func (s *mockServer) Close() error {
	defer os.RemoveAll(s.dir)
	return s.list.Close()
}

func (s *mockServer) Serve(h http.Handler) {
	go func() {
		std.Serve(s.list, h)
	}()
}

func (s *mockServer) Network() string {
	return "unix"
}

func (s *mockServer) Addr() string {
	return filepath.Join(s.dir, "sock")
}

func newServer(t *testing.T) *mockServer {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	sock := filepath.Join(tmp, "sock")

	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}

	return &mockServer{
		list: l,
		dir:  tmp,
		t:    t,
	}
}

func stringSlicesAreSame(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, n := 0, len(a); i < n; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustHaveRequest(
	t *testing.T,
	out io.Reader,
	status int,
	hdrs map[string][]string,
	body []byte) {

	br := bufio.NewReader(out)

	mh, err := textproto.NewReader(br).ReadMIMEHeader()
	if err != nil {
		t.Fatal(err)
	}

	hdr := http.Header(mh)

	s, err := statusFromHeaders(hdr)
	if err != nil {
		t.Fatal(err)
	}

	for k, v := range hdrs {
		m := map[string][]string(hdr)
		if !stringSlicesAreSame(v, m[k]) {
			t.Fatalf("Expected header %s to be %v got %v",
				k, v, hdr[k])
		}
	}

	if s != status {
		t.Fatalf("Expected status %d, got %d", status, s)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, br); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(buf.Bytes(), body) {
		t.Fatalf("expected by of %v got %v", body, buf.Bytes())
	}
}

func paramsFor(verb string,
	params map[string][]string) map[string][]string {
	p := map[string][]string{
		"REQUEST_METHOD":  {verb},
		"SERVER_PROTOCOL": {"HTTP/1.1"},
	}

	for key, vals := range params {
		p[key] = vals
	}

	return p
}

func newRandomData(t *testing.T, n int) []byte {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStatusOK(t *testing.T) {
	s := newServer(t)
	defer s.Close()

	s.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello FCGI")
	}))

	c, err := NewClient(s.Network(), s.Addr())
	if err != nil {
		t.Fatal(err)
	}

	var bout, berr bytes.Buffer
	req, err := c.NewRequest(paramsFor("GET", nil))
	if err != nil {
		t.Fatal(err)
	}

	req.Stdout = &bout
	req.Stderr = &berr

	if err := req.Wait(); err != nil {
		t.Fatal(err)
	}

	mustHaveRequest(t,
		&bout,
		http.StatusOK,
		nil,
		[]byte("Hello FCGI\n"))
}

func TestStatusNotOK(t *testing.T) {
	s := newServer(t)
	defer s.Close()

	s.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Oh No!")
	}))

	c, err := NewClient(s.Network(), s.Addr())
	if err != nil {
		t.Fatal(err)
	}

	var bout, berr bytes.Buffer
	req, err := c.NewRequest(paramsFor("GET", nil))
	if err != nil {
		t.Fatal(err)
	}

	req.Stdout = &bout
	req.Stderr = &berr

	if err := req.Wait(); err != nil {
		t.Fatal(err)
	}

	mustHaveRequest(t,
		&bout,
		http.StatusInternalServerError,
		nil,
		[]byte("Oh No!\n"))
}

func TestWithStdin(t *testing.T) {
	s := newServer(t)
	defer s.Close()

	s.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, r.Body); err != nil {
			t.Fatal(err)
		}
	}))

	c, err := NewClient(s.Network(), s.Addr())
	if err != nil {
		t.Fatal(err)
	}

	var bout, berr bytes.Buffer
	req, err := c.NewRequest(paramsFor("GET", nil))
	if err != nil {
		t.Fatal(err)
	}

	req.Stdin = bytes.NewBufferString("testing\ntesting\ntesting\n")
	req.Stdout = &bout
	req.Stderr = &berr

	if err := req.Wait(); err != nil {
		t.Fatal(err)
	}

	mustHaveRequest(t,
		&bout,
		http.StatusOK,
		nil,
		[]byte("testing\ntesting\ntesting\n"))

}

func TestWithBigStdin(t *testing.T) {
	s := newServer(t)
	defer s.Close()

	s.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, r.Body); err != nil {
			t.Fatal(err)
		}
	}))

	c, err := NewClient(s.Network(), s.Addr())
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, maxWrite+1)

	var bout, berr bytes.Buffer
	req, err := c.NewRequest(paramsFor("GET", nil))
	if err != nil {
		t.Fatal(err)
	}

	req.Stdin = bytes.NewBuffer(buf)
	req.Stdout = &bout
	req.Stderr = &berr

	if err := req.Wait(); err != nil {
		t.Fatal(err)
	}

	mustHaveRequest(t,
		&bout,
		http.StatusOK,
		nil,
		buf)
}

func TestHeaders(t *testing.T) {
	s := newServer(t)
	defer s.Close()

	s.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(knorton): There is a bug in the golang fcgi implementation
		// that fails to retain multi-value headers. It only keeps the last.
		if v := r.Header.Get("X-Foo"); v != "B" {
			t.Fatalf("header X-Foo should be [\"B\"], got %v", v)
		}

		if v := r.Header.Get("X-Bar"); v != "False" {
			t.Fatalf("header X-Bar should be [\"False\"], got %v", v)
		}

		w.Header().Add("X-Foo", "A")
		w.Header().Add("X-Foo", "B")
		w.Header().Set("X-Bar", "False")
	}))

	c, err := NewClient(s.Network(), s.Addr())
	if err != nil {
		t.Fatal(err)
	}

	var bout, berr bytes.Buffer
	req, err := c.NewRequest(
		paramsFor("GET", map[string][]string{
			"HTTP_X_FOO": {"A", "B"},
			"HTTP_X_BAR": {"False"},
		}))
	if err != nil {
		t.Fatal(err)
	}

	req.Stdout = &bout
	req.Stderr = &berr

	if err := req.Wait(); err != nil {
		t.Fatal(err)
	}

	mustHaveRequest(t,
		&bout,
		http.StatusOK,
		map[string][]string{
			"X-Foo": {"A", "B"},
			"X-Bar": {"False"},
		},
		[]byte{})

}

func TestServerTerminatesWithoutTakingStdin(t *testing.T) {
	s := newServer(t)
	defer s.Close()

	go func() {
		c, err := s.list.Accept()
		if err != nil {
			t.Fatal(err)
		}

		var hdr header

		// First read the begin req
		if err := binary.Read(c, binary.BigEndian, &hdr); err != nil {
			t.Fatal(err)
		}

		// next read the content for that request
		b := make([]byte, int(hdr.ContentLength))
		if _, err := io.ReadFull(c, b); err != nil {
			t.Fatal(err)
		}

		// next read the params which will have zero length
		if err := binary.Read(c, binary.BigEndian, &hdr); err != nil {
			t.Fatal(err)
		}

		var w buffer
		w.Reset()

		// now the server abruptly terminates the request
		if err := w.WriteRecord(c, hdr.ID, typeEndRequest); err != nil {
			t.Fatal(err)
		}
	}()

	c, err := NewClient(s.Network(), s.Addr())
	if err != nil {
		t.Fatal(err)
	}

	buf := newRandomData(t, maxWrite+512)

	var bout, berr bytes.Buffer
	req, err := c.NewRequest(nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Stdin = bytes.NewBuffer(buf)
	req.Stdout = &bout
	req.Stderr = &berr

	if err := req.Wait(); err != nil {
		t.Fatal(err)
	}

	if bout.Len() > 0 {
		t.Fatalf("no output expected, got %v", bout.Bytes())
	}
}

func TestAppClosesAbruptly(t *testing.T) {
	s := newServer(t)
	defer s.Close()

	go func() {
		c, err := s.list.Accept()
		if err != nil {
			t.Fatal(err)
		}

		var hdr header

		// First read the begin req
		if err := binary.Read(c, binary.BigEndian, &hdr); err != nil {
			t.Fatal(err)
		}

		// next read the content for that request
		b := make([]byte, int(hdr.ContentLength))
		if _, err := io.ReadFull(c, b); err != nil {
			t.Fatal(err)
		}

		// next read the params which will have zero length
		if err := binary.Read(c, binary.BigEndian, &hdr); err != nil {
			t.Fatal(err)
		}

		c.Close()
	}()

	c, err := NewClient(s.Network(), s.Addr())
	if err != nil {
		t.Fatal(err)
	}

	buf := newRandomData(t, maxWrite+512)

	var bout, berr bytes.Buffer
	req, err := c.NewRequest(nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Stdin = bytes.NewBuffer(buf)
	req.Stdout = &bout
	req.Stderr = &berr

	if err := req.Wait(); err == nil {
		t.Fatal("expected error")
	}
}

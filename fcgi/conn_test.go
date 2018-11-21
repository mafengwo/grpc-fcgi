package fcgi

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestPersistConn_roundTrip(t *testing.T) {
	conn, err := newTestConn()
	if err != nil {
		t.Errorf("dial failed: %v", err)
		return
	}
	t.Log("connected\n")

	req := newRequest()
	resp, err := conn.roundTrip(req)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	t.Logf("response: %v", resp)
}

func TestPersistConn_roundTripReuse(t *testing.T) {
	conn, err := newTestConn()
	if err != nil {
		t.Errorf("dial failed: %v", err)
		return
	}
	t.Log("connected\n")

	for i := 0; i < 10; i++ {
		req := newRequest()
		resp, err := conn.roundTrip(req)
		if err != nil {
			t.Errorf("error: %v", err)
		}
		t.Logf("response: %v", resp)
	}
}

func BenchmarkPersistConn_roundTrip(b *testing.B) {
	conn, err := newTestConn()
	if err != nil {
		b.Fatalf("dial failed: %v", err)
		return
	}
	fmt.Println("\n\nconnected")

	for i :=0; i < b.N; i++ {
		req := newRequest()
		resp, err := conn.roundTrip(req)
		if err != nil {
			b.Errorf("error: %v", err)
		}
		b.Logf("response: %v", resp)
	}
}

func newTestConn() (*persistConn, error) {
	netconn, err := net.Dial("tcp", "127.0.0.1:9000")
	if err != nil {
		return nil, err
	}

	conn := &persistConn{
		conn: netconn,
		reqch:                make(chan requestAndChan, 1),
		writech:              make(chan writeRequestAndError, 1),
		closech:              make(chan struct{}),
		writeErrCh:           make(chan error, 1),
		writeLoopDone:        make(chan struct{}),
	}
	conn.br = bufio.NewReader(conn)
	conn.bw = bufio.NewWriter(conn)

	go conn.readLoop()
	go conn.writeLoop()
	go func() {
		<-conn.closech
		//TODO handle connection closing
	}()
	return conn, nil
}

func newRequest() *Request {
	return &Request{
		Header: map[string][]string{
			"REQUEST_METHOD":    []string{"GET"},
			"SERVER_PROTOCOL":   []string{"HTTP/2.0"},
			"HTTP_HOST":         []string{"localhost"},
			"CONTENT_TYPE":      []string{"text/html"},
			"REQUEST_URI":       []string{"/p1/p2?a=b"},
			"SCRIPT_NAME":       []string{"/p1/p2"},
			"GATEWAY_INTERFACE": []string{"CGI/1.1"},
			"QUERY_STRING":      []string{"a=b"},
			"DOCUMENT_ROOT":     []string{"/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/"},
			"SCRIPT_FILENAME":   []string{"/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/index.php"},
		},
		Body: bytes.NewReader([]byte{0x01}),
	}
}

func TestLoop(t *testing.T) {
	c := make(chan int)
	go func() {
		<-c
		fmt.Println("1")
	}()
	go func() {
		<-c
		fmt.Println("2")
	}()

	close(c)

	<-c
	fmt.Println("3")
	time.Sleep(time.Second * 3)
}
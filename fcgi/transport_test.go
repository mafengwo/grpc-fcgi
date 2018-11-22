package fcgi

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestTransport_RoundTrip(t *testing.T) {
	tran := newTransport()
	resp, err := tran.RoundTrip(newRequest())
	if err != nil {
		t.Errorf(err.Error())
	} else {
		t.Logf("response: %v", resp)
	}
}

func BenchmarkTransport_RoundTrip(b *testing.B) {
	tran := newTransport()
	for i := 0; i < b.N; i++ {
		resp, err := tran.RoundTrip(newRequest())
		if err != nil {
			b.Fatalf(err.Error())
			b.FailNow()
		} else {
			b.Logf("response: %v", resp)
		}
	}
}

func TestTransport_GetConnBlocked(t *testing.T) {
	trans := newTransport()

	unblocked := make(chan bool)
	go func() {
		for i := 0; i < 11; i++ {
			_, err := trans.getConn(nil)
			if err != nil {
				t.Fatalf("get conn error: %v", err)
			}
			t.Logf("index: %d\n", i)
		}
		unblocked <- true
	}()

	select {
	case <-time.After(time.Second * 10):
		t.Logf("10 second past, get 11 conn blocked")
	case <-unblocked:
		t.Errorf("block failed")
	}
}

func TestTransport_GetConnBlockedAndFree(t *testing.T) {
	trans := newTransport()

	unblocked := make(chan bool)
	go func() {
		for i := 0; i < 11; i++ {
			_, err := trans.getConn(nil)
			if err != nil {
				t.Fatalf("get conn error: %v", err)
			}
			t.Logf("index: %d\n", i)
		}
		unblocked <- true
	}()

	go func() {
		trans.decConnCount()
	}()

	select {
	case <-time.After(time.Second * 10):
		t.Logf("10 second past, get 11 conn blocked")
	case <-unblocked:
		t.Errorf("block failed")
	}
}

func newTransport() *Transport {
	return &Transport{
		MaxConn: 1,
		Dial: func(network, addr string) (net.Conn, error) {
			return net.Dial("tcp", "127.0.0.1:9000")
		},
	}
}

func newRequest() *Request {
	return &Request{
		Header: map[string][]string{
			"REQUEST_METHOD":    {"GET"},
			"SERVER_PROTOCOL":   {"HTTP/2.0"},
			"HTTP_HOST":         {"localhost"},
			"CONTENT_TYPE":      {"text/html"},
			"REQUEST_URI":       {"/p1/p2?a=b"},
			"SCRIPT_NAME":       {"/p1/p2"},
			"GATEWAY_INTERFACE": {"CGI/1.1"},
			"QUERY_STRING":      {"a=b"},
			"DOCUMENT_ROOT":     {"/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/"},
			"SCRIPT_FILENAME":   {"/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/index.php"},
		},
		Body: bytes.NewReader([]byte{0x01}),
	}
}


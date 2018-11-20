package fcgi

import (
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
			b.Errorf(err.Error())
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
		MaxConn: 10,
		Dial: func(network, addr string) (net.Conn, error) {
			return net.Dial("tcp", "127.0.0.1:9000")
		},
	}
}

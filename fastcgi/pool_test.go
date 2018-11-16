package fastcgi

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var (
	pool = NewFastcgiClientPool(4, factory)
)

func Test_PoolSend(t *testing.T) {
	header, body, err := pool.Send(newTestParams2(), bytes.NewReader([]byte{0x01}))
	if err != nil {
		t.Errorf("request error: %v", err)
	}

	hjson, _ := json.MarshalIndent(header, "", "    ")
	t.Logf("header:%s\nbody:%s\n", hjson, body)
}

func BenchmarkFastcgiClientPool_SendParallel(b *testing.B) {
	//pool := NewFastcgiClientPool(10, factory)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			header, _, err := pool.Send(newTestParams2(), bytes.NewReader([]byte{0x01}))
			if err != nil {
				b.Errorf("request error: %v", err)
				b.FailNow()
			}

			if len(header) == 0 {
				b.Errorf("content error: %v", err)
				b.FailNow()
			}
			// hjson, _ := json.MarshalIndent(header, "", "    ")
			// b.Logf("header:%s\nbody:%s\n", hjson, body)
		}
	})
}

func BenchmarkClientPool_SendLoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		header, _, err := pool.Send(newTestParams2(), bytes.NewReader([]byte{0x01}))
		if err != nil {
			b.Errorf("request error: %v", err)
			b.FailNow()
		}

		if len(header) == 0  {
			b.Errorf("content error: %v", err)
			b.FailNow()
		}
	}
}

func factory() *Client {
	conf := &ClientConfig{
		Net:         "tcp",
		Addr:        "127.0.0.1:9000",
		DialTimeout: time.Second,
		Timeout:     time.Second,
	}
	return NewClient(conf)
}

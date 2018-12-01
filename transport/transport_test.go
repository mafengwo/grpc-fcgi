package transport

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestTransport_RoundTrip(t *testing.T) {
	tran := newTransport()
	resp, err := tran.roundTrip(newRequest())
	if err != nil {
		t.Errorf(err.Error())
	} else {
		t.Logf("response: %v", resp)
	}
}

func newTransport() *Transport {
	return &Transport{
		MaxConns: 1,
		Address: "127.0.0.1:9000",
	}
}

func newRequest() *Request {
	dc, err := filepath.Abs("./php")
	if err != nil {
		panic("cannot get php path")
	}
	script := dc + "/lite.php"
	r := &Request{
		Header: map[string][]string{
			"REQUEST_METHOD":    {"GET"},
			"SERVER_PROTOCOL":   {"HTTP/2.0"},
			"HTTP_HOST":         {"localhost"},
			"CONTENT_TYPE":      {"text/html"},
			"REQUEST_URI":       {"/p1/p2?a=b"},
			"SCRIPT_NAME":       {"/p1/p2"},
			"GATEWAY_INTERFACE": {"CGI/1.1"},
			"QUERY_STRING":      {"a=b"},
			"DOCUMENT_ROOT":     {dc},
			"SCRIPT_FILENAME":   {script},
		},
		Body: bytes.NewReader([]byte{0x01}),
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Second * 3)
	return r.WithContext(ctx)
}


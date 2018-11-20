package fastcgi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	conf = &ClientConfig{
		Net:         "tcp",
		Addr:        "127.0.0.1:9000",
		DialTimeout: time.Second,
		Timeout:     time.Second,
	}
	cli = NewClient(conf)
)

func Test_Client2Request(t *testing.T) {
	for i := 0; i < 10; i++ {
		header, body, err := cli.Send(newTestParams2(), bytes.NewReader([]byte{0x01}))
		if err != nil {
			t.Errorf("request error: %v", err)
		}

		hjson, _ := json.MarshalIndent(header, "", "    ")
		t.Logf("header:%s\nbody:%s\n", hjson, body)
	}
}

func newTestParams2() map[string][]string {
	return map[string][]string{
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
	}
}

func StatusFromHeaders(h http.Header) (int, error) {
	text := h.Get("Status")
	if text == "" {
		return http.StatusOK, nil
	}

	ix := strings.Index(text, " ")
	if ix >= 0 {
		text = text[:ix]
	}

	s, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, err
	}

	return int(s), nil
}

package mtp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	cli, err = Dial("tcp", "127.0.0.1:9000",
		WithConnectTimeout(3*time.Second),
		WithKeepalive(true),
	)
)

func Test_Request(t *testing.T) {
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	cli, err = Dial("tcp", "127.0.0.1:9000",
		WithConnectTimeout(time.Second),
		WithKeepalive(true),
	)
	if err != nil {
		t.Fatalf("dial fail: %v\n", err)
		return
	} else {
		t.Log("dial success\n")
	}
	for i := 0; i < 10; i++ {
		h, body, err := cli.Request(newTestParams(), strings.NewReader(" "))
		if err != nil {
			t.Errorf("request error: %v", err)
		}

		headerJson, _ := json.MarshalIndent(h, "", "    ")
		t.Logf("header:%s\nbody:%s\n", headerJson, body)
	}
}

func newTestParams() http.Header {
	return map[string][]string{
		"REQUEST_METHOD":    []string{"GET"},
		"SERVER_PROTOCOL":   []string{"HTTP/2.0"},
		"HTTP_HOST":         []string{"localhost"},
		"CONTENT_LENGTH":    []string{strconv.Itoa(int(strings.NewReader(" ").Size()))},
		"CONTENT_TYPE":      []string{"text/html"},
		"REQUEST_URI":       []string{"/p1/p2?a=b"},
		"SCRIPT_NAME":       []string{"/p1/p2"},
		"GATEWAY_INTERFACE": []string{"CGI/1.1"},
		"QUERY_STRING":      []string{"a=b"},
		"DOCUMENT_ROOT":     []string{"/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/"},
		"SCRIPT_FILENAME":   []string{"/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/index.php"},
	}
}

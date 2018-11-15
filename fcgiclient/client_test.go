package fcgiclient

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func Test_Request(t *testing.T) {
	cli, err := Dial("tcp", "127.0.0.1:9000",
		WithConnectTimeout(3*time.Second),
		WithKeepalive(true),
	)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	resp, err := cli.Request(newTestParams(), strings.NewReader(""))
	if err != nil {
		t.Errorf("request error: %v", err)
	}

	respJson, _ := json.MarshalIndent(resp, "", "    ")
	body, _ := ioutil.ReadAll(resp.Body)
	t.Logf("json:%s\nbody:%s\n", respJson, body)
}

func newTestParams() map[string]string {
	return map[string]string{
		"REQUEST_METHOD":    "GET",
		"SERVER_PROTOCOL":   "HTTP/2.0",
		"HTTP_HOST":         "localhost",
		"CONTENT_LENGTH":    "0",
		"CONTENT_TYPE":      "text/html",
		"REQUEST_URI":       "/p1/p2?a=b",
		"SCRIPT_NAME":       "/p1/p2",
		"GATEWAY_INTERFACE": "CGI/1.1",
		"QUERY_STRING":      "a=b",
		"DOCUMENT_ROOT":     "/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/",
		"SCRIPT_FILENAME":   "/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/index.php",
	}
}

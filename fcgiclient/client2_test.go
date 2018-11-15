package fcgiclient

import (
	"bytes"
	"encoding/json"
	"testing"
)

func Test_Client2Request(t *testing.T) {
	cli := &Client2{"127.0.0.1:9000"}

	body, header, err := cli.Send(newTestParams2(), bytes.NewReader([]byte{}))
	if err != nil {
		t.Errorf("request error: %v", err)
	}

	hjson, _ := json.MarshalIndent(header, "", "    ")
	t.Logf("header:%s\nbody:%s\n", hjson, body)
}

func newTestParams2() map[string][]string {
	return map[string][]string{
		"REQUEST_METHOD":    []string{"GET"},
		"SERVER_PROTOCOL":   []string{"HTTP/2.0"},
		"HTTP_HOST":         []string{"localhost"},
		"CONTENT_LENGTH":    []string{"0"},
		"CONTENT_TYPE":      []string{"text/html"},
		"REQUEST_URI":       []string{"/p1/p2?a=b"},
		"SCRIPT_NAME":       []string{"/p1/p2"},
		"GATEWAY_INTERFACE": []string{"CGI/1.1"},
		"QUERY_STRING":      []string{"a=b"},
		"DOCUMENT_ROOT":     []string{"/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/"},
		"SCRIPT_FILENAME":   []string{"/Users/jjw/gocode/src/gitlab.mfwdev.com/golibrary/grpc-proxy-test/php/index.php"},
	}
}

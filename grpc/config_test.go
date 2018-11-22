package grpc

import (
	"io/ioutil"
	"os"
	"testing"
)

var (
	config1 = `
address: "0.0.0.0:8080"
timeout: 60
queue_size: 1000
reserve_headers:
  - "Content-Type"

fastcgi:
  address: "127.0.0.1:9000"
  connection_limit: 10
  script_file_name: "/var/html/index.php"
  document_root: "/var/html"

`
)

func Test_LoadConfig(t *testing.T) {
	td := os.TempDir()+"/"+"grpc_proxy_config.yml"
	defer os.RemoveAll(td)

	err := ioutil.WriteFile(td, []byte(config1), 0777)
	if err != nil {
		t.Fatalf("write config file failed: %v", err)
	}

	opt, err := LoadConfig(td)
	if err != nil {
		t.Fatalf("write config file failed: %v", err)
	}
}
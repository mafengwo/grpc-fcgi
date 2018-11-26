package grpc

import (
	"github.com/bakins/grpc-fastcgi-proxy/log"
	"path/filepath"
	"testing"
)

var (
	dc, _ = filepath.Abs("../fcgi/php/")
	opt = &Options{
		Address:        "0.0.0.0:8080",
		QueueSize:      100,
		Timeout:        60,
		ReserveHeaders: []string{"Content-Type"},
		Fcgi: FcgiOptions{
			Address:         "127.0.0.1:9000",
			ConnectionLimit: 10,
			ScriptFileName:  dc + "/index.php",
			DocumentRoot:    dc,
		},
	}

	logopt = &log.Options{
		AccessLogPath: "stdout",
		ErrorLogPath:  "stderr",
		ErrorLogLevel: "debug",
	}
)

func TestProxy_Serve(t *testing.T) {
	logger, err := log.NewLogger(logopt)
	if err != nil {
		t.Fatalf("cannot init logger: %v", err)
	}

	p, err := NewProxy(opt, logger)
	if err != nil {
		t.Fatalf("cannot init proxy: %v", err)
	}

	if err = p.Serve(); err != nil {
		t.Fatalf("failed to serve: %v", err)
	}
}

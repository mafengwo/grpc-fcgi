package grpc

import (
	"gitlab.mfwdev.com/service/grpc-fcgi/log"
	"io/ioutil"
	"os"
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
		ErrorLogLevel: "warn",
		ErrorLogTrace: false,
	}
)

func TestProxy_Serve(t *testing.T) {
	p := NewProxy(opt, prepareLogger())
	startServe(p)
}

var (
	costlyPhp = `<?php

time.Sleep(50);
`
)
func TestProxy_Timeout(t *testing.T) {
	release := writeTempPhpFile(costlyPhp, opt)
	defer release()

	p := NewProxy(opt, prepareLogger())
	startServe(p)
}

var (
	fastAndSlowPhp = `<?php

if ($us = rand(1000, 10000000)) {
	usleep($us);
}
`
)

func TestProxy_FastAndSlow(t *testing.T) {
	release := writeTempPhpFile(fastAndSlowPhp, opt)
	defer release()

	p := NewProxy(opt, prepareLogger())
	startServe(p)
}

func writeTempPhpFile(content string, opt *Options) func() {
	fp := dc + "/test.php"
	if err := ioutil.WriteFile(fp, []byte(content), 0777); err != nil {
		panic("write file failed: " + err.Error())
	}
	opt.Fcgi.ScriptFileName = fp

	return func() {
		os.Remove(fp)
	}
}

func prepareLogger() *log.Logger {
	logger, err := log.NewLogger(logopt)
	if err != nil {
		panic("cannot init logger: " + err.Error())
	}
	return logger
}

func startServe(p *Proxy) {
	if err := p.Serve(); err != nil {
		panic("failed to serve: " + err.Error())
	}
}


package grpc

import (
	"github.com/jinzhu/configor"
	"github.com/pkg/errors"
	"net"
)

type LogOptions struct {
	AccessLogPath string `required:"true" yaml:"access_log_path"`
	ErrorLogPath  string `required:"true" yaml:"error_log_path"`
	ErrorLogLevel string `required:"true" yaml:"error_log_level"`
	ErrorLogTrace bool   `yaml:"error_log_trace"`
}

type FcgiOptions struct {
	Address              string `required:"true" yaml:"address"`
	ConnectionLimit      int    `required:"true" yaml:"connection_limit"`
	ConnectionMaxRequest int    `required:"true" yaml:"connection_max_request"`
	ScriptFileName       string `required:"true" yaml:"script_file_name"`
	DocumentRoot         string `required:"true" yaml:"document_root"`
}

type Options struct {
	Address        string   `required:"true" yaml:"address"`
	QueueSize      int      `required:"true" yaml:"queue_size"`
	Timeout        int      `required:"true" yaml:"timeout"`
	ReserveHeaders []string `yaml:"reserve_headers"`

	Fcgi FcgiOptions `required:"true" yaml:"fastcgi"`
	Log  LogOptions  `required:"true" yaml:"log"`
}

func LoadConfig(file string) (*Options, error) {
	opt := &Options{}
	err := configor.Load(opt, file)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load configuration")
	}
	return opt, nil
}

func canonicalizateHostPort(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.Wrap(err, "SplitHostPort failed")
	}
	return net.JoinHostPort(host, port), nil
}

func (opt *Options) Validate() error {
	return nil
}

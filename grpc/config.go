package grpc

import (
	"fmt"
	"github.com/jinzhu/configor"
	"github.com/pkg/errors"
	"net"
)

var (
	allowedLogLevels = []string{"error", "warn", "info", "debug"}
)

type LogOptions struct {
	AccessLogPath string `required:"true" yaml:"access_log_path" json:"access_log_path"`
	ErrorLogPath  string `required:"true" yaml:"error_log_path" json:"error_log_path"`
	ErrorLogLevel string `required:"true" yaml:"error_log_level" json:"error_log_level"`
}

func (opt *LogOptions) validate() *ConfigItemError {
	levelCollect := false
	for _, v := range allowedLogLevels {
		if v == opt.ErrorLogLevel {
			levelCollect = true
			break
		}
	}
	if !levelCollect {
		return &ConfigItemError{"error_log_level",
		errors.New("error_log_level must be one of [error warn info debug]")}
	}

	return nil
}

type FcgiOptions struct {
	Address        string `required:"true" yaml:"address" json:"address"`
	MaxConns       int    `required:"true" yaml:"max_connections" json:"max_connections"`
	MaxIdleConns   int    `required:"true" yaml:"max_idle_connections" json:"max_idle_connections"`
	ScriptFileName string `required:"true" yaml:"script_file_name" json:"script_file_name"`
	DocumentRoot   string `required:"true" yaml:"document_root" json:"document_root"`
}

func (opt *FcgiOptions) validate() *ConfigItemError {
	// validate fastcgi config items
	faddr, err := canonicalizateHostPort(opt.Address)
	if err != nil {
		return &ConfigItemError{"address", err}
	}
	opt.Address = faddr

	if opt.MaxIdleConns <= 0 || opt.MaxIdleConns > opt.MaxConns {
		return &ConfigItemError{
			"max_idle_connections",
			errors.New(fmt.Sprintf(
				"max_idle_connections must be a positive number, and cannot larger than max_connections"))}
	}

	if opt.MaxConns <= 0 {
		return &ConfigItemError{"max_connections",
			errors.New(fmt.Sprintf("max_connections must be a positive number"))}
	}

	return nil
}

type Options struct {
	Address      string `required:"true" yaml:"address" json:"address"`
	Timeout      int    `required:"true" yaml:"timeout" json:"timeout"`
	PprofAddress string `required:"true" yaml:"pprof_address" json:"pprof_address"`

	Fcgi FcgiOptions `required:"true" yaml:"fastcgi" json:"fastcgi"`
	Log  LogOptions  `required:"true" yaml:"log" json:"log"`
}

func LoadConfig(file string) (*Options, error) {
	opt := &Options{}
	err := configor.Load(opt, file)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load configuration")
	}
	return opt, nil
}

func (opt *Options) Validate() *ConfigItemError {
	addr, err := canonicalizateHostPort(opt.Address)
	if err != nil {
		return &ConfigItemError{"address", err}
	}
	opt.Address = addr

	pprofAddr, err := canonicalizateHostPort(opt.PprofAddress)
	if err != nil {
		return &ConfigItemError{"pprof_address", err}
	}
	opt.PprofAddress = pprofAddr

	if opt.Timeout <= 0 {
		return &ConfigItemError{"timeout", errors.New(fmt.Sprintf("timeout must be a positive number"))}
	}


	if ferr := opt.Fcgi.validate(); ferr != nil {
		ferr.Path = ferr.Path.withParent("fastcgi")
		return ferr
	}

	// validate logging
	if ferr := opt.Log.validate(); ferr != nil {
		ferr.Path = ferr.Path.withParent("log")
		return ferr
	}

	return nil
}

type path string

func (p path) withParent(parent path) path {
	return path(parent + "." + p)
}

type ConfigItemError struct {
	Path path
	Err  error
}

func(ie *ConfigItemError) Error() string {
	return fmt.Sprintf("config item `%s` error: %v", ie.Path, ie.Err)
}

func canonicalizateHostPort(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.Wrap(err, "SplitHostPort failed")
	}
	return net.JoinHostPort(host, port), nil
}
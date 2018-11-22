package grpc

import (
	"github.com/pkg/errors"
	"net"
)

type FcgiOptions struct {
	Address         string `required:"true"`
	ConnectionLimit int    `required:"true"`
	ScriptFileName  string `required:"true"`
	DocumentRoot    string `required:"true"`
}

type Options struct {
	Address        string `required:"true"`
	QueueSize      int    `required:"true"`
	Timeout        int    `required:"true"`
	ReserveHeaders []string

	Fcgi FcgiOptions
}

func LoadConfig(file string) (*Options, error) {

}

func canonicalizateHostPort(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.Wrap(err, "SplitHostPort failed")
	}
	return net.JoinHostPort(host, port), nil
}

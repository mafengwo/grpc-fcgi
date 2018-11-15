package grpc

import (
	"github.com/pkg/errors"
	"net"
)

type Options struct {
	Address        string `required:"true"`
	Concurrency    int    `required:"true"`
	Timeout        int    `required:"true"`
	HeadersAllowed []string

	FastcgiAddress        string `required:"true"`
	FastcgiScriptFileName string `required:"true"`
	FastcgiDocumentRoot   string `required:"true"`
}

func (o *Options) Validate() error {

}

func canonicalizateHostPort(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.Wrap(err, "SplitHostPort failed")
	}
	return net.JoinHostPort(host, port), nil
}

package grpc

import (
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/pkg/errors"
	"github.com/mafengwo/grpc-fcgi/fcgi"
	"github.com/mafengwo/grpc-fcgi/log"
	"google.golang.org/grpc"
	"net"
	"net/http"
)

type Proxy struct {
	opt *Options

	fcgiClient *fcgi.Transport

	internalServer *grpc.Server

	logger *log.Logger
}

func NewProxy(opt *Options, logger *log.Logger) *Proxy {
	p := &Proxy{
		opt:    opt,
		logger: logger,
		fcgiClient: &fcgi.Transport{
			MaxConns:     opt.Fcgi.MaxConns,
			MaxIdleConns: opt.Fcgi.MaxIdleConns,
			Address:      opt.Fcgi.Address,
		},
	}

	p.internalServer = grpc.NewServer(
		grpc.CustomCodec(Codec()),
		grpc.UnknownServiceHandler(p.handleStream),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				grpc_recovery.StreamServerInterceptor(),
			),
		),
	)

	return p
}

func (p *Proxy) Serve() error {
	l, err := net.Listen("tcp", p.opt.Address)
	if err != nil {
		return errors.Wrapf(err, "failed to listen on %s", p.opt.Address)
	}

	if err := p.internalServer.Serve(l); err != nil {
		if err != http.ErrServerClosed {
			return errors.Wrapf(err, "failed to server grpc server")
		}
	}
	return nil
}

func (p *Proxy) GracefulStop() error {
	//TODO timeout
	p.internalServer.GracefulStop()
	return nil
}

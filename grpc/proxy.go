package grpc

import (
	"github.com/bakins/grpc-fastcgi-proxy/fcgiclient"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"net"
	"net/http"

)

type Proxy struct {
	opt               *Options
	fastcgiClientPool *fcgiclient.FastcgiClientPool
	internalServer    *grpc.Server
}

func NewProxy(opt *Options) (*Proxy, error) {

}

func (p *Proxy) Serve() error {
	grpc_zap.ReplaceGrpcLogger(s.logger)

	p.internalServer = grpc.NewServer(
		grpc.CustomCodec(Codec()),
		grpc.UnknownServiceHandler(p.handleStream),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				grpc_zap.StreamServerInterceptor(s.logger),
				grpc_recovery.StreamServerInterceptor(),
			),
		),
	)

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


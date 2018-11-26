package grpc

import (
	"context"
	"github.com/bakins/grpc-fastcgi-proxy/fcgi"
	"github.com/bakins/grpc-fastcgi-proxy/log"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net"
	"net/http"
)

type Proxy struct {
	opt            *Options
	streamHandler  *streamHandler
	internalServer *grpc.Server
	logger         *log.Logger
}

func NewProxy(opt *Options, logger *log.Logger) (*Proxy, error) {
	sh := &streamHandler{
		fcgiOptions: &opt.Fcgi,
		fcgiClient: &fcgi.Transport{
			MaxConn: opt.Fcgi.ConnectionLimit,
			Dial: func(network, addr string) (net.Conn, error) {
				return net.Dial("tcp", opt.Fcgi.Address)
			},
		},
		errorLogger: logger.AcquireErrorLogger(),
		logAccess: func(fields ...zap.Field) {
			logger.AcquireAccessLogger().Info("", fields...)
		},
	}
	p := &Proxy{
		opt:  opt,
		streamHandler: sh,
		logger:logger,
	}
	p.internalServer = grpc.NewServer(
		grpc.CustomCodec(Codec()),
		grpc.UnknownServiceHandler(sh.handleStream),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				p.StreamInterceptorContext,
				grpc_recovery.StreamServerInterceptor(),
			),
		),
	)
	return p, nil
}

func (p *Proxy) Serve() error {
	// grpc_zap.ReplaceGrpcLogger(s.logger)
	p.logger.AcquireErrorLogger().Info("starting to serve")

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

func (p *Proxy)StreamInterceptorContext(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	newCtx := context.WithValue(stream.Context(), "access_logger", p.logger.AcquireErrorLogger())
	wrapped := grpc_middleware.WrapServerStream(stream)
	wrapped.WrappedContext = newCtx

	err := handler(srv, wrapped)
	return err
}

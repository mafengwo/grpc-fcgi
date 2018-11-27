package grpc

import (
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/pkg/errors"
	"gitlab.mfwdev.com/service/grpc-fcgi/fcgi"
	"gitlab.mfwdev.com/service/grpc-fcgi/log"
	"google.golang.org/grpc"
	"net"
	"net/http"
	"time"
)

type Proxy struct {
	opt            *Options
	streamHandler  *streamHandler
	internalServer *grpc.Server
	logger         *log.Logger
}

func NewProxy(opt *Options, logger *log.Logger) *Proxy {
	sh := &streamHandler{
		fcgiOptions: &opt.Fcgi,
		fcgiClient: &fcgi.Transport{
			MaxConn: opt.Fcgi.ConnectionLimit,
			Dial: func(network, addr string) (net.Conn, error) {
				return net.Dial("tcp", opt.Fcgi.Address)
			},
		},
		reservedHeaders: opt.ReserveHeaders,
		logger:          logger,
	}
	if opt.QueueSize > 0 {
		sh.queue = make(chan int, opt.QueueSize)
	}
	if opt.Timeout > 0 {
		sh.timeout = time.Second * time.Duration(opt.Timeout)
	}

	p := &Proxy{
		opt:           opt,
		streamHandler: sh,
		logger:        logger,
	}
	p.internalServer = grpc.NewServer(
		grpc.CustomCodec(Codec()),
		grpc.UnknownServiceHandler(sh.handleStream),
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

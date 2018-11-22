package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	// For profiling
	_ "net/http/pprof"

	// To register gzip compressor
	_ "google.golang.org/grpc/encoding/gzip"

	"github.com/bakins/grpc-fastcgi-proxy/internal/errgroup"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Server is an http/2 server that proxies to fastcgi
type Server struct {
	/*
		address           string
		auxAddress        string
		fastEndpoint      *url.URL
		entryFile         string
		docRoot           string
		auxPaths          map[string]string //paths we will serve on aux port
	*/

	options Options

	httpServer        *http.Server
	grpcServer        *grpc.Server
	fastcgiClientPool *fastcgiClientPool

	logger *zap.Logger
}

// NewServer creates a new Server.
func NewServer(options Options) *Server {
	s := &Server{
		options:           options,
		httpServer:        newHttpServer(options.HttpOpt),
		grpcServer:        newGrpcServer(options.GrpcOpt),
		fastcgiClientPool: newFastcgiClientPool(options.FastcgiOpt),
		logger:            newLogger(options.LogOpt),
	}
	return s

	/*
		if s.logger == nil {
			l, err := NewLogger()
			if err != nil {
				return nil, errors.Wrap(err, "failed to create logger")
			}
			s.logger = l
		}

		s.fastcgiClientPool = newFastcgiClientPool(s.fastEndpoint, 4)

		return s, nil
	*/
}

// Run starts the server. Generally this never returns.
func (s *Server) Run() error {

	grpc_zap.ReplaceGrpcLogger(s.logger)
	// TODO - config option for this
	grpc_prometheus.EnableHandlingTimeHistogram()

	s.grpcServer = grpc.NewServer(
		grpc.CustomCodec(Codec()),
		grpc.UnknownServiceHandler(s.streamHandler),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				grpc_zap.StreamServerInterceptor(s.logger),
				grpc_recovery.StreamServerInterceptor(),
			),
		),
	)

	g := errgroup.New()

	g.Go(func() error {
		l, err := net.Listen("tcp", s.address)
		if err != nil {
			return errors.Wrapf(err, "failed to listen on %s", s.address)
		}

		if err := s.grpcServer.Serve(l); err != nil {
			if err != http.ErrServerClosed {
				return errors.Wrapf(err, "failed to server grpc server: %s", s.address)
			}
		}
		return nil
	})

	s.httpServer = &http.Server{
		Addr:    s.auxAddress,
		Handler: http.DefaultServeMux,
	}

	g.Go(func() error {
		if err := s.httpServer.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				return errors.Wrap(err, "failed to start http server")
			}
		}
		return nil
	})

	if err := g.Wait(); err == nil {
		return errors.Wrap(err, "failed to start servers")
	}
	return nil
}

// Stop will stop the server
func (s *Server) Stop() {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.grpcServer.GracefulStop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}()

	wg.Wait()
}

/*
func canonicalizateHostPort(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.Wrap(err, "SplitHostPort failed")
	}
	return net.JoinHostPort(host, port), nil
}

// SetAddress creates a function that will set the listening address.
// Generally, used when create a new Server.
func SetAddress(addr string) func(*Server) error {
	return func(s *Server) error {
		a, err := canonicalizateHostPort(addr)
		if err != nil {
			return errors.Wrapf(err, "failed to parse address: %s", addr)
		}
		s.address = a
		return nil
	}
}

// SetAuxAddress creates a function that will set the aux address.
// Generally, used when create a new Server.
func SetAuxAddress(addr string) func(*Server) error {
	return func(s *Server) error {
		a, err := canonicalizateHostPort(addr)
		if err != nil {
			return errors.Wrapf(err, "failed to parse address: %s", addr)
		}
		s.auxAddress = a
		return nil
	}
}

// SetFastCGIEndpoint creates a function that will set the fastCGI endpoint
// to proxy. Endpoint should be url.
// Default is tcp://127.0.0.1:9090
// Generally, used when create a new Server.
func SetFastCGIEndpoint(endpoint string) func(*Server) error {
	return func(s *Server) error {
		u, err := url.Parse(endpoint)
		if err != nil {
			return errors.Wrapf(err, "failed to parse endpoint: %s", endpoint)
		}

		//XXX: if using tcp, ensure we have a host and port?
		s.fastEndpoint = u
		return nil
	}
}

// SetLogger creates a function that will set the logger.
// Generally, used when create a new Server.
func SetLogger(l *zap.Logger) func(*Server) error {
	return func(s *Server) error {
		s.logger = l
		return nil
	}
}

// SetEntryFile creates a function that will set the entryfile for php.
// Generally, used when create a new Server.
func SetEntryFile(f string) func(*Server) error {
	return func(s *Server) error {
		s.entryFile = f
		s.docRoot = path.Dir(f)
		return nil
	}
}

// AddAuxPath adds a path to be served on the auxiliary port.
// if filename is empty, then the current working directory
// plus path is used
func (s *Server) AddAuxPath(path string, filename string) error {
	// TODO: ensure the path is not registered
	// either here or by pprof, metrics, etc
	if filename == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "unable to get working directory")
		}
		filename = filepath.Join(cwd, path)
	}

	abs, err := filepath.Abs(filename)
	if err != nil {
		return errors.Wrapf(err, "unable to determine absolute path of %s", filename)
	}
	s.auxPaths[path] = abs

	return nil
}

*/

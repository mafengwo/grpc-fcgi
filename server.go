package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	// For profiling
	_ "net/http/pprof"

	"github.com/bakins/grpc-fastcgi-proxy/internal/errgroup"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Server is an http/2 server that proxies to fastcgi
type Server struct {
	address           string
	auxAddress        string
	fastEndpoint      *url.URL
	entryFile         string
	docRoot           string
	httpServer        *http.Server
	grpcServer        *grpc.Server
	logger            *zap.Logger
	fastcgiClientPool *fastcgiClientPool
	auxPaths          map[string]string //paths we will serve on aux port
}

// OptionsFunc is a function passed to New to set options.
type OptionsFunc func(*Server) error

// NewServer creates a new Server.
func NewServer(options ...OptionsFunc) (*Server, error) {
	s := &Server{
		address:    "127.0.0.1:8080",
		auxAddress: "127.0.0.1:7070",
		fastEndpoint: &url.URL{
			Scheme: "tcp",
			Host:   "127.0.0.1:9090",
		},
		auxPaths: map[string]string{},
	}

	for _, f := range options {
		if err := f(s); err != nil {
			return nil, errors.Wrap(err, "options failed")
		}
	}

	if s.logger == nil {
		l, err := NewLogger()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create logger")
		}
		s.logger = l
	}

	s.fastcgiClientPool = newFastcgiClientPool(s.fastEndpoint, 4)

	return s, nil
}

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
				grpc_prometheus.StreamServerInterceptor,
				grpc_zap.StreamServerInterceptor(s.logger),
				grpc_recovery.StreamServerInterceptor(),
			),
		),
	)

	// TODO: allow setting these paths and check
	// that there is no conflict with passthrough paths
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", s.healthz)

	for path, filename := range s.auxPaths {
		http.Handle(path, s.auxPathHandle(filename))
	}

	g := errgroup.New()

	g.Go(func() error {
		l, err := net.Listen("tcp", s.address)
		if err != nil {
			return errors.Wrapf(err, "failed to listen on %s", s.address)
		}

		if err := s.grpcServer.Serve(l); err != nil {
			if err != http.ErrServerClosed {
				return errors.Wrapf(err, "failed to server grpc server", s.address)
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

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK\n")
}

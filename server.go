package proxy

import (
	"net"
	"net/http"
	"path"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/hkwi/h2c"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Server is an http/2 server that proxies to fastcgi
type Server struct {
	address          string
	fastEndpoint     string
	entryFile        string
	docRoot          string
	server           *http.Server
	grpc             *grpc.Server
	logger           *zap.Logger
	client           *client
	passthroughPaths []string // paths that we pass through to fastcgi even if not fastcgi
}

// TODO: TLS support

// OptionsFunc is a function passed to New to set options.
type OptionsFunc func(*Server) error

// NewServer creates a new Server.
func NewServer(options ...OptionsFunc) (*Server, error) {
	s := &Server{
		address:      "127.0.0.1:8080",
		fastEndpoint: "127.0.0.1:9090",
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

	s.client = newClient(s, s.fastEndpoint, 4)

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

// SetFastCGIEndpoint creates a function that will set the fastCGI endpoint
// to proxy.
// Generally, used when create a new Server.
func SetFastCGIEndpoint(addr string) func(*Server) error {
	return func(s *Server) error {
		a, err := canonicalizateHostPort(addr)
		if err != nil {
			return errors.Wrapf(err, "failed to parse address: %s", addr)
		}
		s.fastEndpoint = a
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

// SetPassthroughPaths creates a function that will set paths
// that will be passed through to fastcgi, even if not grpc.
// Generally, used when create a new Server.
func SetPassthroughPaths(paths []string) func(*Server) error {
	return func(s *Server) error {

		// a few paths are reserved
		for _, p := range paths {
			switch p {
			case "/metrics":
				return errors.New("/metrics is used for prometheus metrics")
			default:
			}
		}

		s.passthroughPaths = paths
		return nil
	}
}

// Run starts the server. Generally this never returns.
func (s *Server) Run() error {

	grpc_zap.ReplaceGrpcLogger(s.logger)
	// TODO - config option for this
	grpc_prometheus.EnableHandlingTimeHistogram()

	s.grpc = grpc.NewServer(
		grpc.CustomCodec(Codec()),
		grpc.UnknownServiceHandler(s.streamHandler),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				grpc_prometheus.UnaryServerInterceptor,
				grpc_zap.UnaryServerInterceptor(s.logger),
				grpc_recovery.UnaryServerInterceptor(),
			),
		),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				grpc_prometheus.StreamServerInterceptor,
				grpc_zap.StreamServerInterceptor(s.logger),
				grpc_recovery.StreamServerInterceptor(),
			),
		),
	)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	for _, p := range s.passthroughPaths {
		mux.HandleFunc(p, s.nonGrpcHandle)
	}

	l, err := net.Listen("tcp", s.address)
	if err != nil {
		return errors.Wrapf(err, "failed to listen on %s", s.address)
	}

	s.server = &http.Server{
		Handler: h2c.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.ProtoMajor == 2 &&
					strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
					s.grpc.ServeHTTP(w, r)
				} else {
					mux.ServeHTTP(w, r)
				}
			}),
		},
	}

	if err := s.server.Serve(l); err != nil {
		if err != http.ErrServerClosed {
			return errors.Wrap(err, "failed to start http server")
		}
	}

	return nil
}

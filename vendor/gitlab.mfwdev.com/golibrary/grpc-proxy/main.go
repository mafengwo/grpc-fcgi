package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/transport"

	"gitlab.mfwdev.com/golibrary/grpc-proxy/proxy"
)

var (
	ErrApplicationShutdown = errors.New("Application is shutting down")
	configFile string
)

func start() error {
	flag.StringVar(&configFile, "f", "", "config file")
	flag.Parse()

	options, loadErr := proxy.LoadConfig(configFile)
	if loadErr != nil {
		return loadErr
	}

	debugSigChan := make(chan os.Signal, 10)
	signal.Notify(debugSigChan, syscall.SIGUSR2)
	defer func() {
		signal.Stop(debugSigChan)
		close(debugSigChan)
	}()
	// kill -USR2 PID 可以切换debug模式的开关
	go func() {
		for range debugSigChan {
			if logrus.GetLevel() == logrus.DebugLevel {
				logrus.SetLevel(logrus.InfoLevel)
				getLogger().Info("debug log is turned off")
			} else {
				logrus.SetLevel(logrus.DebugLevel)
				getLogger().Debug("debug log is turned on")
			}
		}
	}()

	if options.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		getLogger().Debug("debug log is turned on")
	}

	var err error
	var lis net.Listener
	// 通过TCP协议监听服务端口
	// 暴露的gRPC服务是否使用TLS认证
	if options.CrtFile != "" && options.KeyFile != "" {
		getLogger().Infof("using TLS from (%s, %s)", options.CrtFile, options.KeyFile)

		cer, readErr := tls.LoadX509KeyPair(options.CrtFile, options.KeyFile)
		if readErr != nil {
			return errors.WithMessage(readErr, "failed to load key pair")
		}

		config := &tls.Config{
			ClientAuth:               tls.VerifyClientCertIfGiven,
			InsecureSkipVerify:       true,
			PreferServerCipherSuites: true,
			Certificates:             []tls.Certificate{cer},
			NextProtos:               []string{"h2"},
		}
		lis, err = tls.Listen("tcp", options.Host, config)
	} else {
		lis, err = net.Listen("tcp", options.Host)
	}

	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to listen on %s", options.Host))
	}
	defer lis.Close()
	getLogger().WithField("host", options.Host).Info("binding successful")

	// 通过soheilhy/cmux开源实现进行grpc服务和http服务的多路复用
	trafficMux := cmux.New(lis)
	//grpcL := trafficMux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	grpcL := trafficMux.Match(cmux.HTTP2())
	httpL := trafficMux.Match(cmux.HTTP1Fast())

	group, ctx := errgroup.WithContext(context.Background())

	// 初始化GRPCProxy实例.
	proxy := proxy.GRPCProxy{
		Logger: getLogger(),
		Handler: func(ctx context.Context, ts *transport.Stream, t transport.ServerTransport) {
			h := proxy.StreamHandler{
				Logger:        getLogger(),
				PortalOptions: options.Target,
			}
			h.HandleStream(ctx, ts, t)
		},
	}

	// 负责响应os.Interrupt信号
	// 关闭grpc监听，退出进程。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	group.Go(func() error {
		<-sigCh
		grpcL.Close()
		return ErrApplicationShutdown
	})

	// 把grpc监听到的请求转发给GRPCProxy实例进行处理
	group.Go(func() error {
		getLogger().Info("starting serve")
		err = proxy.Serve(ctx, grpcL)
		if err != nil {
			return errors.WithMessage(err, "failed to serve")
		}
		return nil
	})

	// start http health check
	// 启动http的监控检查服务.
	group.Go(func() error {
		getLogger().Info("starting http serve")

		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "This grpc entry point - please use grpc requests")
			w.WriteHeader(http.StatusMethodNotAllowed)
		})

		httpServer := &http.Server{Handler: mux}
		if err := httpServer.Serve(httpL); err != nil {
			return errors.WithMessage(err, "failed to serve http")
		}

		return nil
	})

	group.Go(func() error {
		getLogger().Info("starting tcp mux")
		if err := trafficMux.Serve(); err != nil {
			return errors.WithMessage(err, "failed to serve tcp mux")
		}

		return nil
	})

	return group.Wait()
}

// Test using http://localhost:8089/v1/test/wait?time=10
func main() {
	logger := getLogger()
	logrus.SetLevel(logrus.InfoLevel)
	logger.Info("proxy is starting up ")
	err := start()
	if err != nil && errors.Cause(err) != ErrApplicationShutdown {
		logger.WithError(err).Error("application fail")
	} else {
		logger.Info("successful application shutdown. Goodbye!")
	}
}

func getLogger() logrus.FieldLogger {
	return logrus.StandardLogger()
}

package main

import (
	"flag"
	"gitlab.mfwdev.com/service/grpc-fcgi/grpc"
	"gitlab.mfwdev.com/service/grpc-fcgi/log"
	"os"
	"os/signal"
	"syscall"
)

var (
	configFile = flag.String( "f", "", "config file path")
)

func main() {
	flag.Parse()

	options, loadErr := grpc.LoadConfig(*configFile)
	if loadErr != nil {
		panic("cannot load config file: " + loadErr.Error())
	}

	if err := options.Validate(); err != nil {
		panic("config item is illegal: " + err.Error())
	}

	logger, err := log.NewLogger(&log.Options{
		AccessLogPath: options.Log.AccessLogPath,
		ErrorLogPath: options.Log.ErrorLogPath,
		ErrorLogLevel: options.Log.ErrorLogLevel,
		ErrorLogTrace: options.Log.ErrorLogTrace,
	})
	if err != nil {
		panic("cannot build logger: " + err.Error())
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	s := grpc.NewProxy(options, logger)
	go func() {
		<-sigs
		logger.ErrorLogger().Info("signal received, prepare to shutdown...")
		s.GracefulStop()
		logger.ErrorLogger().Info("server has been shutdown")
	}()

	logger.ErrorLogger().Info("starting to serve")
	if err := s.Serve(); err != nil {
		logger.ErrorLogger().Error("serve failed: " + err.Error())
		os.Exit(3)
	}
}

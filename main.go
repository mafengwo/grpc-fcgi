package main

import (
	"encoding/json"
	"flag"
	"github.com/mafengwo/grpc-fcgi/grpc"
	"github.com/mafengwo/grpc-fcgi/log"
	"go.uber.org/zap"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
)

var (
	configFile = flag.String("f", "", "config file path")
)

func main() {
	flag.Parse()

	// load config items
	options, loadErr := grpc.LoadConfig(*configFile)
	if loadErr != nil {
		panic("cannot load config file: " + loadErr.Error())
	}
	if err := options.Validate(); err != nil {
		panic("config item is illegal: " + err.Error())
	}

	// initialize logger
	logger, err := log.NewLogger(&log.Options{
		AccessLogPath: options.Log.AccessLogPath,
		ErrorLogPath:  options.Log.ErrorLogPath,
		ErrorLogLevel: options.Log.ErrorLogLevel,
	})
	if err != nil {
		panic("cannot build logger: " + err.Error())
	}

	// logging config for feedback
	optjson, err := json.MarshalIndent(options, "", "  ")
	if err != nil {
		panic("marshal config failed:" + err.Error())
	}
	logger.ErrorLogger().Info(string(optjson), zap.String("type", "config_items"))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	s := grpc.NewProxy(options, logger)
	go func() {
		<-sigs
		logger.ErrorLogger().Info("signal received, prepare to shutdown...")
		s.GracefulStop()
		logger.ErrorLogger().Info("server has been shutdown")
	}()

	// pprof
	go func() {
		// for healthy check
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Write([]byte("ok"))
			}
		})

		err := http.ListenAndServe(options.PprofAddress, nil)
		logger.ErrorLogger().Info("serve pprof: " + err.Error())
	}()

	logger.ErrorLogger().Info("starting to serve")
	if err := s.Serve(); err != nil {
		logger.ErrorLogger().Error("serve failed: " + err.Error())
		os.Exit(3)
	}
}

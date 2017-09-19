package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	proxy "github.com/bakins/grpc-fastcgi-proxy"
	"github.com/spf13/cobra"
)

var (
	addr             *string
	fastcgi          *string
	passthroughPaths []string
)

var rootCmd = &cobra.Command{
	Use:   "grpc-fastcgi-proxy",
	Short: "grpc to fastcgi proxy",
	Run:   runServer,
}

func runServer(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println("entryfile is required")
		os.Exit(-4)
	}

	logger, err := proxy.NewLogger()
	if err != nil {
		panic(err)
	}

	s, err := proxy.NewServer(
		proxy.SetAddress(*addr),
		proxy.SetFastCGIEndpoint(*fastcgi),
		proxy.SetLogger(logger),
		proxy.SetEntryFile(args[0]),
		proxy.SetPassthroughPaths(passthroughPaths),
	)

	if err != nil {
		logger.Fatal("unable to create server", zap.Error(err))
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		s.Stop()
	}()

	if err := s.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func main() {
	f := rootCmd.PersistentFlags()
	addr = f.StringP("address", "a", "127.0.0.1:8080", "listen address")
	fastcgi = f.StringP("fastcgi", "f", "127.0.0.1:9000", "fastcgi to proxy")
	f.StringSliceVar(&passthroughPaths, "passthrough", []string{}, "paths that will be passed through to fastcgi on non-grpc requests")
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

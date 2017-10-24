package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.uber.org/zap"

	proxy "github.com/bakins/grpc-fastcgi-proxy"
	"github.com/spf13/cobra"
)

var (
	auxAddr  *string
	addr     *string
	fastcgi  *string
	auxPaths []string
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
		proxy.SetAuxAddress(*auxAddr),
		proxy.SetFastCGIEndpoint(*fastcgi),
		proxy.SetLogger(logger),
		proxy.SetEntryFile(args[0]),
	)

	for _, val := range auxPaths {
		parts := strings.SplitN(val, "=", 2)
		path := parts[0]
		filename := ""
		if len(parts) == 2 && parts[1] != "" {
			filename = parts[1]
		}

		if err := s.AddAuxPath(path, filename); err != nil {
			logger.Fatal("failed to add auxillary path",
				zap.Error(err),
				zap.String("value", val),
			)
		}
	}

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
	auxAddr = f.StringP("aux-address", "x", "127.0.0.1:7070", "aux listen address")
	fastcgi = f.StringP("fastcgi", "f", "tcp://127.0.0.1:9000", "fastcgi to proxy")
	f.StringSliceVar(&auxPaths, "aux-path", []string{}, "paths to pass to fastcgi when accessed on the aux port. example: /path=filename")
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

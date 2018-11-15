package proxy

import (
	"flag"
	"fmt"
	"github.com/bakins/grpc-fastcgi-proxy/grpc"
	"gitlab.mfwdev.com/golibrary/grpc-proxy/proxy"
	"os"
	"os/signal"
	"syscall"
)

var (
	configFile string
)

type options struct {
	grpcOpt *grpc.Options
}

func main() {
	flag.StringVar(&configFile, "f", "", "config file path")
	flag.Parse()

	options, loadErr := loadConfig(configFile)
	if loadErr != nil {
		//TODO panic
	}

	if err := options.Validate(); err != nil {
		//TODO panic
	}

	s, err := proxy.NewServer(options)
	if err != nil {
		//TODO panic
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

func loadConfig(path string) (*options, error) {

}

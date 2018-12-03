# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

BINARY_MAC=bin/grpc_fastcgi_proxy_darwin
BINARY_LINUX=bin/grpc_fastcgi_proxy_linux

pkg:
	dep ensure

test:
	$(GOTEST)

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_MAC) -v

clean:
	$(GOCLEAN)
	rm -f $(BINARY_MAC)
	rm -f $(BINARY_LINUX)

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_LINUX) -v
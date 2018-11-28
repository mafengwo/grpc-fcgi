# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

GLIDECMD=glide
GLIDEUPDATE=$(GLIDECMD) update

BINARY_NAME=bin/grpc-proxy
BINARY_UNIX=$(BINARY_NAME)_unix

all: build
glide:
	$(GLIDEUPDATE)
test:
	$(GOTEST)
build:
	$(GOBUILD) -o $(BINARY_NAME) -v
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v

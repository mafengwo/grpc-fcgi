package grpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/bakins/grpc-fastcgi-proxy/fcgi"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strings"
	"time"
)

type request struct {
	requestID     string
	requestTime   time.Time
	host          string
	method        string
	requestLength int
	headers       map[string][]string
	body          *bytes.Reader

	roundTripTime  time.Duration
	upstreamTime   time.Duration
	status         int
	upstreamStatus int
	bodyBytesSent  int

	logger *zap.Logger
	ctx    context.Context
}

func readRequest(stream grpc.ServerStream, r *request) error {
	method, exist := grpc.MethodFromServerStream(stream)
	if !exist {
		return errors.New("failed to extract method from stream")
	}
	r.method = method

	f := &frame{}
	if err := stream.RecvMsg(f); err != nil {
		return errors.New(fmt.Sprintf("RecvMsg failed: %s", err))
	}
	r.body = bytes.NewReader(f.payload)

	h := map[string][]string{}
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return errors.New("failed to extract metadata")
	}
	for k, v := range md {
		h["HTTP_"+strings.Replace(strings.ToUpper(k), "-", "_", -1)] = []string{v[0]}
	}
	r.headers = h

	r.host = "host" //todo
	r.requestLength = int(r.body.Size())
	return nil
}

func (r *request) toFcgiRequest(opts *FcgiOptions) (*fcgi.Request) {
	h := map[string][]string{
		"REQUEST_METHOD":    {"POST"},
		"SERVER_PROTOCOL":   {"HTTP/2.0"},
		"HTTP_HOST":         {"localhost"}, // reserve host grpc request
		"CONTENT_TYPE":      {"text/html"},
		"REQUEST_URI":       {r.method},
		"SCRIPT_NAME":       {r.method},
		"GATEWAY_INTERFACE": {"CGI/1.1"},
		"QUERY_STRING":      {""},
		"DOCUMENT_ROOT":     {opts.DocumentRoot},
		"SCRIPT_FILENAME":   {opts.ScriptFileName},
	}

	return &fcgi.Request{
		Header: h,
		Body:   r.body,
	}
}

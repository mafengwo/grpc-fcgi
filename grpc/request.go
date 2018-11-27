package grpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"gitlab.mfwdev.com/service/grpc-fcgi/fcgi"
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
	metadata       map[string][]string
	body          *bytes.Reader

	roundTripTime  time.Duration
	upstreamTime   time.Duration
	status         int
	upstreamStatus int
	bodyBytesSent  int

	ctx    context.Context

	accessLogger   *zap.Logger
	errorLogger *zap.Logger
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

	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return errors.New("failed to extract metadata")
	}
	for k, v := range md {
		if k == ":authority" && len(v) != 0 {
			r.host = v[0]
			delete(md, k)
		}
	}
	r.metadata = md

	r.requestLength = int(r.body.Size())
	return nil
}

func (r *request) toFcgiRequest(opts *FcgiOptions) (*fcgi.Request) {
	h := map[string][]string{
		"REQUEST_METHOD":    {"POST"},
		"SERVER_PROTOCOL":   {"HTTP/2.0"},
		"HTTP_HOST":         {r.host}, // reserve host grpc request
		"CONTENT_TYPE":      {"text/html"},
		"REQUEST_URI":       {r.method},
		"SCRIPT_NAME":       {r.method},
		"GATEWAY_INTERFACE": {"CGI/1.1"},
		"QUERY_STRING":      {""},
		"DOCUMENT_ROOT":     {opts.DocumentRoot},
		"SCRIPT_FILENAME":   {opts.ScriptFileName},
	}
	for k, v := range r.metadata {
		h["HTTP_"+strings.Replace(strings.ToUpper(k), "-", "_", -1)] = []string{v[0]}
	}

	req := &fcgi.Request{
		Header: h,
		Body:   r.body,
	}
	return req.WithContext(r.ctx)
}

package grpc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gitlab.mfwdev.com/service/grpc-fcgi/fcgi"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net/http/httptrace"
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
	body          []byte

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

	r.body = f.payload[:]
	r.requestLength = int(len(r.body))

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

	return nil
}

func getRequestId(stream grpc.ServerStream) string {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if ok {
		if id, exist := md["request_id"]; exist && len(id) > 0 {
			return id[0]
		}
	}

	return uuid.New().String()
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

	ct := &httptrace.ClientTrace{
		GetConn: func(hostPort string){
			r.accessLogger = r.accessLogger.With(zap.Time("GetConn", time.Now()))
		},
		GotConn: func(conn httptrace.GotConnInfo) {
			r.accessLogger = r.accessLogger.With(zap.Time("GotConn", time.Now()))
		},
		PutIdleConn: func(err error) {
			r.accessLogger = r.accessLogger.With(zap.Time("PutIdle", time.Now()))
			if err != nil {
				r.accessLogger = r.accessLogger.With(zap.String("PutIdleError", err.Error()))
			}
		},
		GotFirstResponseByte: func() {
			r.accessLogger = r.accessLogger.With(zap.Time("GotFirstResponseByte", time.Now()))
		},
		ConnectStart: func(network, addr string) {
			r.accessLogger = r.accessLogger.With(zap.Time("ConnectStart", time.Now()))
		},
		ConnectDone: func(network, addr string, err error) {
			r.accessLogger = r.accessLogger.With(zap.Time("ConnectDone", time.Now()))
		},
		WroteHeaders: func() {
			r.accessLogger = r.accessLogger.With(zap.Time("WroteHeaders", time.Now()))
		},
		WroteRequest: func(reqInfo httptrace.WroteRequestInfo) {
			r.accessLogger = r.accessLogger.With(zap.Time("WroteRequest", time.Now()))
		},
	}
	ctx := httptrace.WithClientTrace(r.ctx, ct)

	return req.WithContext(ctx).WithRequestId(r.requestID)
}

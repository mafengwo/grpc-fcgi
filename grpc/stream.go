package grpc

import (
	"context"
	"fmt"
	"github.com/bakins/grpc-fastcgi-proxy/fcgi"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net/http"
	"strings"
	"time"
)

type streamHandler struct {
	fcgiOptions *FcgiOptions
	fcgiClient  *fcgi.Transport
	errorLogger *zap.Logger
	logAccess   func(fields ...zap.Field)
}

func (sh *streamHandler) handleStream(srv interface{}, stream grpc.ServerStream) error {
	req := &request{
		requestID: uuid.New().String(),
		requestTime: time.Now(),
	}
	defer sh.log(req)

	if err := readRequest(stream, req); err != nil {
		sh.errorLogger.Error("failed to parse stream to fcgi request:" + err.Error())
		return status.Errorf(codes.Internal, err.Error())
	}

	clientCtx, clientCancel := context.WithCancel(stream.Context())
	defer clientCancel()

	fcgiReq := req.toFcgiRequest(sh.fcgiOptions)
	fcgiReq = fcgiReq.WithContext(clientCtx)
	sh.errorLogger.Debug(fmt.Sprintf("fastcgi request: %v", fcgiReq.Header))

	resp, err := sh.fcgiClient.RoundTrip(fcgiReq)
	sh.errorLogger.Debug(fmt.Sprintf("fastcgi response: %v", resp.Headers))
	if err != nil {
		return status.Errorf(codes.Internal, "fastcgi request failed: %s", err)
	}

	return sh.sendResponse(stream, resp)
}

func (sh *streamHandler) sendResponse(stream grpc.ServerStream, resp *fcgi.Response) error {
	// TODO: convert resp code to grpc code
	statusCode, err := resp.GetStatusCode()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to parse status code of fcgi response: %s", err)
	}
	if statusCode != http.StatusOK {
		return status.Errorf(codes.Internal, string(resp.Body))
	}

	responseMetadata := metadata.MD{}
	for k, v := range sh.filterToGrpcHeaders(resp.Headers) {
		responseMetadata[strings.ToLower(k)] = v
	}
	if err := stream.SendHeader(responseMetadata); err != nil {
		return status.Errorf(codes.Internal, "failed to send headers: %s", err)
	}

	responseFrame := frame{
		payload: resp.Body,
	}
	if err := stream.SendMsg(&responseFrame); err != nil {
		return status.Errorf(codes.Internal, "failed to send message: %s", err)
	}

	return nil
}

func (sh *streamHandler) filterToGrpcHeaders(fcgiHeaders map[string][]string) map[string][]string {
	// this probably need to be munged?
	return fcgiHeaders
}

func (sh *streamHandler) log(req *request) {
	sh.logAccess(zap.String("request_id", req.requestID),
		zap.Time("time", req.requestTime),
		zap.String("host", req.host),
		zap.String("request_uri", req.method),
		zap.Int("request_length", req.requestLength),
		zap.Duration("request_time", req.roundTripTime),
		zap.Duration("upstream_time", req.upstreamTime),
		zap.Int("status", req.status),
		zap.Int("upstream_status", req.upstreamStatus),
		zap.Int("body_bytes_sent", req.bodyBytesSent),
	)
}

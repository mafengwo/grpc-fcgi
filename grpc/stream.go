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

	queue           chan int
	timeout         time.Duration
	reservedHeaders []string
}

func (sh *streamHandler) handleStream(srv interface{}, stream grpc.ServerStream) error {
	req := &request{
		requestID:   uuid.New().String(),
		requestTime: time.Now(),
	}
	defer sh.log(req)

	var cancelc context.CancelFunc
	if sh.timeout > 0 {
		req.ctx, cancelc = context.WithTimeout(stream.Context(), sh.timeout)
	} else {
		req.ctx, cancelc = context.WithCancel(stream.Context())
	}
	defer cancelc()

	proxyDone := make(chan error)
	go func() {
		err := sh.handleRequest(stream, req)
		proxyDone <- err
	}()

	select {
	case <-req.ctx.Done():
		sh.errorLogger.Warn("request context done")
		return status.Errorf(codes.DeadlineExceeded, "context deadline exceeded")
	case err := <-proxyDone:
		req.status = int(codes.DeadlineExceeded)
		if err != nil {
			sh.errorLogger.Error(fmt.Sprintf("execute request failed: %v", err))
		}
		sh.errorLogger.Debug("request execute done")
		return err
	}
}

func (sh *streamHandler) handleRequest(stream grpc.ServerStream, req *request) error {
	if err := readRequest(stream, req); err != nil {
		sh.errorLogger.Error("failed to read request from request:" + err.Error())
		return status.Errorf(codes.Internal, "failed to read request from request: %v", err)
	}

	release := sh.waitingForProxy()
	defer release()

	dl, dlset := req.ctx.Deadline()
	if dlset && dl.Before(time.Now()) {
		return status.Errorf(codes.DeadlineExceeded, "context deadline exceeded after waiting")
	}

	fcgiReq := req.toFcgiRequest(sh.fcgiOptions)
	sh.errorLogger.Debug(fmt.Sprintf("fastcgi request: %v", fcgiReq.Header))

	// proxy to fastcgi server
	resp, err := sh.fcgiClient.RoundTrip(fcgiReq)
	sh.errorLogger.Debug(fmt.Sprintf("fastcgi response: %v", resp.Headers))
	if err != nil {
		return status.Errorf(codes.Internal, "failed to proxy: %v", err)
	}

	// read information about response
	req.upstreamTime = time.Now().Sub(req.requestTime)
	req.bodyBytesSent = len(resp.Body)
	statusCode, err := resp.GetStatusCode()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to parse status code of fcgi response: %s", err)
	}
	req.upstreamStatus = statusCode

	s := sh.sendResponse(stream, resp)
	req.roundTripTime = time.Now().Sub(req.requestTime)
	req.status = int(s.Code())
	return status.Errorf(s.Code(), s.Message())
}

func (sh *streamHandler) waitingForProxy() func() {
	if sh.queue != nil {
		sh.queue <- 1
	}
	return func() {
		if sh.queue != nil {
			<-sh.queue
		}
	}
}

func (sh *streamHandler) sendResponse(stream grpc.ServerStream, resp *fcgi.Response) *status.Status {
	// TODO: convert resp code to grpc code
	statusCode, err := resp.GetStatusCode()
	if err != nil {
		return status.Newf(codes.Internal, "failed to parse status code of fcgi response: %s", err)
	}
	if statusCode != http.StatusOK {
		return status.Newf(codes.Internal, "got fastcgi status: %d", statusCode)
	}

	responseMetadata := metadata.MD{}
	for k, v := range sh.filterToGrpcHeaders(resp.Headers) {
		responseMetadata[strings.ToLower(k)] = v
	}
	if err := stream.SendHeader(responseMetadata); err != nil {
		return status.Newf(codes.Internal, "failed to send headers: %s", err)
	}

	responseFrame := frame{
		payload: resp.Body,
	}
	if err := stream.SendMsg(&responseFrame); err != nil {
		return status.Newf(codes.Internal, "failed to send message: %s", err)
	}

	return status.Newf(codes.OK, "")
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

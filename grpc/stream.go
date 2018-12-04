package grpc

import (
	"context"
	"fmt"
	"gitlab.mfwdev.com/service/grpc-fcgi/fcgi"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net/http"
	"strings"
	"time"
)

func (p *Proxy) handleStream(srv interface{}, stream grpc.ServerStream) error {
	reqid := getRequestId(stream)
	req := &request{
		requestID:    reqid,
		requestTime:  time.Now(),
		fcgiRounds:   &fcgiRequestRounds{rounds: []*fcgiRequestRound{}},
		accessLogger: p.logger.NewAccessLogger(),
		errorLogger:  p.logger.NewErrorLogger().With(zap.String("request_id", reqid)),
	}
	defer req.logAccess()

	req.errorLogger.Debug("request received")

	var cancelc context.CancelFunc
	if p.opt.Timeout > 0 {
		req.ctx, cancelc = context.WithTimeout(stream.Context(), time.Duration(p.opt.Timeout)*time.Second)
	} else {
		req.ctx, cancelc = context.WithCancel(stream.Context())
	}
	defer cancelc()

	proxyDone := make(chan *status.Status)
	go func() {
		s := p.handleRequest(stream, req)
		proxyDone <- s
	}()

	var result *status.Status
	select {
	case <-req.ctx.Done():
		req.errorLogger.Debug("request context deadline exceeded")
		result = status.Newf(codes.DeadlineExceeded, "context deadline exceeded")
	case result = <-proxyDone:
		if result.Code() != codes.OK {
			req.errorLogger.Error(fmt.Sprintf("execute request failed: %v", result.Message()))
		}
	}

	req.sentResponse = time.Now()
	req.status = int(result.Code())

	return status.Error(result.Code(), result.Message())
}

func (p *Proxy) handleRequest(stream grpc.ServerStream, req *request) *status.Status {
	if err := readRequest(stream, req); err != nil {
		req.errorLogger.Error("failed to read request from request:" + err.Error())
		return status.Newf(codes.Internal, "failed to read request from request: %v", err)
	}

	dl, dlset := req.ctx.Deadline()
	if dlset && dl.Before(time.Now()) {
		return status.Newf(codes.DeadlineExceeded, "context deadline exceeded after waiting")
	}

	// proxy to fastcgi server
	var resp *fcgi.Response
	var err error
	retrying := true
	for retrying {
		fcgiReq := req.toFcgiRequest(p.opt.Fcgi)
		if p.opt.Log.AccessLogTrace {
			fcgiReq = fcgiReq.WithTrace()
		}

		round := req.rotateRound(fcgiReq)
		resp, retrying, err = p.fcgiClient.RoundTrip(fcgiReq)
		round.resp = resp
		round.err = err
	}
	if err != nil {
		return status.Newf(codes.Internal, "failed to proxy: %v", err)
	}

	// read information about the final response
	req.bodyBytesSent = len(resp.Body)

	return p.sendResponse(stream, resp)
}

func (p *Proxy) sendResponse(stream grpc.ServerStream, resp *fcgi.Response) *status.Status {
	// TODO: convert resp code to grpc code
	statusCode, err := resp.GetStatusCode()
	if err != nil {
		return status.Newf(codes.Internal, "failed to parse status code of fcgi response: %s", err)
	}
	if statusCode != http.StatusOK {
		return status.Newf(codes.Internal, "got fastcgi status: %d", statusCode)
	}

	responseMetadata := metadata.MD{}
	for k, v := range p.filterToGrpcHeaders(resp.Headers) {
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

func (p *Proxy) filterToGrpcHeaders(fcgiHeaders map[string][]string) map[string][]string {
	// this probably need to be munged?
	return fcgiHeaders
}

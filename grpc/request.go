package grpc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gitlab.mfwdev.com/service/grpc-fcgi/fcgi"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strings"
	"time"
)

type fcgiRequestRound struct {
	req  *fcgi.Request
	resp *fcgi.Response
	err  error
}

func (r *fcgiRequestRound) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	if !r.req.Trace.ConnectStartTime.IsZero() {
		enc.AddTime("connect_start_time", r.req.Trace.ConnectStartTime)
	}
	if !r.req.Trace.ConnectDoneTime.IsZero() {
		enc.AddTime("connect_done_time", r.req.Trace.ConnectDoneTime)
	}
	if !r.req.Trace.GetConnTime.IsZero() {
		enc.AddTime("get_connection_time", r.req.Trace.GetConnTime)
	}
	if !r.req.Trace.GotConnTime.IsZero() {
		enc.AddTime("got_connection_time", r.req.Trace.GotConnTime)
	}
	if !r.req.Trace.WroteHeadersTime.IsZero() {
		enc.AddTime("wrote_headers_time", r.req.Trace.WroteHeadersTime)
	}
	if !r.req.Trace.WroteRequestTime.IsZero() {
		enc.AddTime("wrote_request_time", r.req.Trace.WroteRequestTime)
	}
	if !r.req.Trace.GotFirstResponseByteTime.IsZero() {
		enc.AddTime("got_first_response_byte_time", r.req.Trace.GotFirstResponseByteTime)
	}
	if !r.req.Trace.PutIdleTime.IsZero() {
		enc.AddTime("put_idle_connection_time", r.req.Trace.PutIdleTime)
	}
	if r.req.Trace.PutIdleError != nil {
		enc.AddString("put_idle_connection_error", r.req.Trace.PutIdleError.Error())
	}

	if r.resp != nil {
		status, err := r.resp.GetStatusCode()
		if err != nil {
			enc.AddString("response_status_code_error", err.Error())
		}

		enc.AddInt("response_status_code", status)
	}

	if r.err != nil {
		enc.AddString("error", r.err.Error())
	}

	return nil
}

type fcgiRequestRounds struct {
	rounds []*fcgiRequestRound
}

func (rs *fcgiRequestRounds) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, r := range rs.rounds {
		err := enc.AppendObject(r)
		if err != nil {
			return err
		}
	}
	return nil
}

type request struct {
	requestID         string
	host              string
	method            string
	requestBodyLength int
	metadata          map[string][]string
	body              []byte
	bodyBytesSent     int

	requestTime  time.Time
	sentResponse time.Time

	status int

	fcgiRounds *fcgiRequestRounds

	ctx context.Context

	accessLogger *zap.Logger
	errorLogger  *zap.Logger
}

func (r *request) toFcgiRequest(opts *FcgiOptions) (*fcgi.Request) {
	h := map[string][]string{
		"REQUEST_METHOD":  {"POST"},
		"CONTENT_TYPE":    {"application/grpc"},
		"REQUEST_URI":     {r.method},
		"DOCUMENT_ROOT":   {opts.DocumentRoot},
		"SCRIPT_FILENAME": {opts.ScriptFileName},
	}
	for k, v := range r.metadata {
		h["HTTP_"+strings.Replace(strings.ToUpper(k), "-", "_", -1)] = []string{v[0]}
	}

	req := &fcgi.Request{
		Header: h,
		Body:   r.body,
		Ctx:    r.ctx,
	}
	return req.WithRequestId(r.requestID).WithTrace()
}

func (r *request) rotateRound(fcgiReq *fcgi.Request) (*fcgiRequestRound) {
	round := &fcgiRequestRound{req: fcgiReq}
	r.fcgiRounds.rounds = append(r.fcgiRounds.rounds, round)
	return round
}

func (r *request) logAccess() {
	fields := []zap.Field{
		zap.String("request_id", r.requestID),
		zap.Time("request_time", r.requestTime),
		zap.String("host", r.host),
		zap.String("request_uri", r.method),
		zap.Int("request_body_length", r.requestBodyLength),
		zap.Duration("round_trip_time", r.sentResponse.Sub(r.requestTime)),
		zap.Int("status", r.status),
		zap.Int("body_bytes_sent", r.bodyBytesSent),
		zap.Array("trace", r.fcgiRounds),
	}
	r.accessLogger.Info("", fields...)
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
	r.requestBodyLength = int(len(r.body))

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

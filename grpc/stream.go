package grpc

import (
	"bytes"
	"context"
	"github.com/bakins/grpc-fastcgi-proxy/fcgi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/transport"
	"net/http"
	"strings"
)

func (p *Proxy) handleStream(srv interface{}, stream grpc.ServerStream) error {
	clientCtx, clientCancel := context.WithCancel(stream.Context())
	defer clientCancel()

	req, err := p.parseStreamToFcgiRequest(stream)
	if err != nil {
		return err
	}
	req = req.WithContext(clientCtx)

	resp, err := p.fcgi.RoundTrip(req)
	if err != nil {
		return status.Errorf(codes.Internal, "fastcgi request failed: %s", err)
	}

	return p.sendResponse(stream, resp)

}

func (p *Proxy) parseStreamToFcgiRequest(stream grpc.ServerStream) (*fcgi.Request, error) {
	f := &frame{}
	if err := stream.RecvMsg(f); err != nil {
		return nil, status.Errorf(codes.Internal, "RecvMsg failed: %s", err)
	}

	h, err := p.getFcgiHeaders(stream)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ParseHeaders failed: %s", err)
	}

	return &fcgi.Request{
		Header: h,
		Body:   bytes.NewReader(f.payload),
	}, nil
}

func (p *Proxy) getFcgiHeaders(stream grpc.ServerStream) (map[string][]string, error) {
	lowLevelServerStream, ok := transport.StreamFromContext(stream.Context())
	if !ok {
		return nil, status.Errorf(codes.Internal, "lowLevelServerStream does not exist in context")
	}

	h := map[string][]string{
		"REQUEST_METHOD":    {"POST"},
		"SERVER_PROTOCOL":   {"HTTP/2.0"},
		"HTTP_HOST":         {"localhost"}, // reserve host grpc request
		"CONTENT_TYPE":      {"text/html"},
		"REQUEST_URI":       {lowLevelServerStream.Method()},
		"SCRIPT_NAME":       {lowLevelServerStream.Method()},
		"GATEWAY_INTERFACE": {"CGI/1.1"},
		"QUERY_STRING":      {lowLevelServerStream.Method()},
		"DOCUMENT_ROOT":     {p.opt.Fcgi.DocumentRoot},
		"SCRIPT_FILENAME":   {p.opt.Fcgi.ScriptFileName},
	}

	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return nil, status.Errorf(codes.Internal, "failed to extract metadata")
	}
	for k, v := range md {
		h["HTTP_"+strings.Replace(strings.ToUpper(k), "-", "_", -1)] = []string{v[0]}
	}
	return h, nil
}

func (p *Proxy)sendResponse(stream grpc.ServerStream, resp *fcgi.Response) error {
	// TODO: convert resp code to grpc code
	statusCode, err := resp.GetStatusCode()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to parse status code of fcgi response: %s", err)
	}
	if statusCode != http.StatusOK {
		return status.Errorf(codes.Internal, string(resp.Body))
	}

	responseMetadata := metadata.MD{}
	for k, v := range p.filterToGrpcHeaders(resp.Headers) {
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

func (p *Proxy) filterToGrpcHeaders(fcgiHeaders map[string][]string) map[string][]string {
	// this probably need to be munged?
	return fcgiHeaders
}

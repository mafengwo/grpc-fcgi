package grpc

import (
	"bytes"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/transport"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

func (p *Proxy) handleStream(srv interface{}, stream grpc.ServerStream) error {
	lowLevelServerStream, ok := transport.StreamFromContext(stream.Context())
	if !ok {
		return status.Errorf(codes.Internal, "lowLevelServerStream does not exist in context")
	}

	fullMethodName := lowLevelServerStream.Method()

	clientCtx, clientCancel := context.WithCancel(stream.Context())
	defer clientCancel()

	f := &frame{}
	if err := stream.RecvMsg(f); err != nil {
		return status.Errorf(codes.Internal, "RecvMsg failed: %s", err)
	}
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Errorf(codes.Internal, "failed to extract metadata")
	}

	req := &http.Request{
		Method: "POST",
		URL: &url.URL{
			Scheme: "http",
			Path:   fullMethodName,
		},
		Proto:         "HTTP/2.0",
		ProtoMajor:    2,
		ProtoMinor:    0,
		Header:        make(http.Header),
		ContentLength: int64(len(f.payload)),
		Body:          ioutil.NopCloser(bytes.NewBuffer(f.payload)),
	}

	req = req.WithContext(clientCtx)

	addRequestHeaders(md, req)

	params := p.paramsFromRequest(req)

	resp, err := p.fastcgiClientPool.Request(params, req.Body)

	if err != nil {
		return status.Errorf(codes.Internal, "fastcgi request failed: %s", err)
	}

	// TODO: convert resp code to grpc code
	if resp.code != http.StatusOK {
		return status.Errorf(codes.Internal, string(resp.body))
	}

	responseFrame := frame{
		payload: resp.body,
	}

	responseMetadata := metadata.MD{}

	for k, v := range resp.header {
		// this probably need to be munged?
		responseMetadata[strings.ToLower(k)] = v
	}

	if err := stream.SendHeader(responseMetadata); err != nil {
		return status.Errorf(codes.Internal, "failed to send headers: %s", err)
	}

	if err := stream.SendMsg(&responseFrame); err != nil {
		return status.Errorf(codes.Internal, "failed to send message: %s", err)
	}

	return nil
}

func (p *Proxy) paramsFromRequest(r *http.Request) map[string]string {
	params := map[string]string{
		"REQUEST_METHOD":    r.Method,
		"SERVER_PROTOCOL":   fmt.Sprintf("HTTP/%d.%d", r.ProtoMajor, r.ProtoMinor),
		"HTTP_HOST":         r.Host,
		"CONTENT_LENGTH":    fmt.Sprintf("%d", r.ContentLength),
		"CONTENT_TYPE":      r.Header.Get("Content-Type"),
		"REQUEST_URI":       r.URL.Path,
		"SCRIPT_NAME":       r.URL.Path,
		"GATEWAY_INTERFACE": "CGI/1.1",
		"QUERY_STRING":      r.URL.RawQuery,
		"DOCUMENT_ROOT":     p.opt.FastcgiDocumentRoot,
		"SCRIPT_FILENAME":   p.opt.FastcgiScriptFileName,
	}

	for k, v := range r.Header {
		params["HTTP_"+strings.Replace(strings.ToUpper(k), "-", "_", -1)] = v[0]
	}

	return params
}

func addRequestHeaders(md metadata.MD, req *http.Request) {
	host := "localhost"

	for k, v := range md {
		if k == ":authority" && len(v) != 0 {
			host = v[0]
		} else {
			for _, val := range v {
				req.Header.Add(k, val)
			}
		}
	}
	req.Host = host
	req.Header.Set("Host", host)
	req.URL.Host = host
}

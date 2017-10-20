// based on https://github.com/mwitkow/grpc-proxy
// Apache 2 License by Michal Witkowski (mwitkow)

package proxy

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/transport"
)

func (s *Server) streamHandler(srv interface{}, stream grpc.ServerStream) error {
	lowLevelServerStream, ok := transport.StreamFromContext(stream.Context())
	if !ok {
		return grpc.Errorf(codes.Internal, "lowLevelServerStream does not exist in context")
	}

	fullMethodName := lowLevelServerStream.Method()

	clientCtx, clientCancel := context.WithCancel(stream.Context())
	defer clientCancel()

	f := &frame{}
	if err := stream.RecvMsg(f); err != nil {
		return grpc.Errorf(codes.Internal, "RecvMsg failed: %s", err)
	}
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return grpc.Errorf(codes.Internal, "failed to extract metadata")
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

	env := map[string]string{
		"DOCUMENT_ROOT":   s.docRoot,
		"SCRIPT_FILENAME": s.entryFile,
	}

	resp, err := s.fastcgiClientPool.request(req, env)

	if err != nil {
		return grpc.Errorf(codes.Internal, "fastcgi request failed: %s", err)
	}

	// TODO: convert resp code to grpc code
	if resp.code != http.StatusOK {
		return grpc.Errorf(codes.Internal, string(resp.body))
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
		return grpc.Errorf(codes.Internal, "failed to send headers: %s", err)
	}

	if err := stream.SendMsg(&responseFrame); err != nil {
		return grpc.Errorf(codes.Internal, "failed to send message: %s", err)
	}

	return nil
}

func (s *Server) auxPathHandle(path string, filename string) http.Handler {

	docroot := filepath.Dir(filename)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env := map[string]string{
			"DOCUMENT_ROOT":   docroot,
			"SCRIPT_FILENAME": filename,
		}
		resp, err := s.fastcgiClientPool.request(r, env)

		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		w.WriteHeader(resp.code)
		for k, v := range resp.header {
			for _, val := range v {
				r.Header.Add(k, val)
			}
		}
		_, _ = w.Write(resp.body)
	})
}

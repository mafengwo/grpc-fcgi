// based on https://github.com/mwitkow/grpc-proxy
// Apache 2 License by Michal Witkowski (mwitkow)

package proxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/transport"
)

// Codec returns a proxying grpc.Codec with the default protobuf codec as parent.
//
// See CodecWithParent.
func Codec() grpc.Codec {
	return CodecWithParent(&protoCodec{})
}

// CodecWithParent returns a proxying grpc.Codec with a user provided codec as parent.
//
// This codec is *crucial* to the functioning of the proxy. It allows the proxy server to be oblivious
// to the schema of the forwarded messages. It basically treats a gRPC message frame as raw bytes.
// However, if the server handler, or the client caller are not proxy-internal functions it will fall back
// to trying to decode the message using a fallback codec.
func CodecWithParent(fallback grpc.Codec) grpc.Codec {
	return &rawCodec{fallback}
}

type rawCodec struct {
	parentCodec grpc.Codec
}

type frame struct {
	payload []byte
}

func (c *rawCodec) Marshal(v interface{}) ([]byte, error) {
	out, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Marshal(v)
	}
	return out.payload, nil

}

func (c *rawCodec) Unmarshal(data []byte, v interface{}) error {
	dst, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Unmarshal(data, v)
	}
	dst.payload = data
	return nil
}

func (c *rawCodec) String() string {
	return fmt.Sprintf("proxy>%s", c.parentCodec.String())
}

// protoCodec is a Codec implementation with protobuf. It is the default rawCodec for gRPC.
type protoCodec struct{}

func (protoCodec) Marshal(v interface{}) ([]byte, error) {
	return proto.Marshal(v.(proto.Message))
}

func (protoCodec) Unmarshal(data []byte, v interface{}) error {
	return proto.Unmarshal(data, v.(proto.Message))
}

func (protoCodec) String() string {
	return "proto"
}

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
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
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

	resp, err := s.client.request(req, s.entryFile, nil)

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

func (s *Server) passthroughHandle(w http.ResponseWriter, r *http.Request) {
	env := map[string]string{
		"DOCUMENT_ROOT":   s.docRoot,
		"SCRIPT_FILENAME": filepath.Join(s.docRoot, r.URL.Path),
	}
	resp, err := s.client.request(r, s.entryFile, env)

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
}

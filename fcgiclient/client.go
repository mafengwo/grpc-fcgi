package fcgiclient

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/kellegous/fcgi"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strconv"
)

type Config struct {
	Address     string
	DialTimeout int
	Timeout     int
}

type Client struct {
	Address string
}

type SizedReader interface {
	io.Reader
	Size() int64
}

func (c *Client) Send(p map[string][]string, sr SizedReader) ([]byte, http.Header, error) {
	client, err := fcgi.NewClient("tcp", c.Address)
	if err != nil {
		return nil, nil, err
	}

	wout := &bytes.Buffer{}
	werr := &bytes.Buffer{}

	req, err := client.NewRequest(p)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to begin FCGI request")
	}

	req.Stdin = sr
	req.Stdout = wout
	req.Stderr = werr

	if err = req.Wait(); err != nil {
		return nil, nil, errors.WithMessage(err, "failed to wait for FCGI response")
	}


	out, err := ioutil.ReadAll(wout)
	fmt.Printf("output: %s", out)

	rb := bufio.NewReader(wout)
	tp := textproto.NewReader(rb)

	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to parse FCGI response header")
	}
	header := http.Header(mimeHeader)

	// TODO: fixTransferEncoding ?
	transferEncoding := header["Transfer-Encoding"]
	//contentLength, _ := strconv.ParseInt(header.Get("Content-Length"), 10, 64)

	var result []byte

	if chunked(transferEncoding) {
		result, err = ioutil.ReadAll(httputil.NewChunkedReader(rb))
		if err != nil {
			return nil, nil, errors.WithMessage(err, "failed to parse FCGI chunked response")
		}
	} else {
		result, err = ioutil.ReadAll(rb)
		if err != nil {
			return nil, nil, errors.WithMessage(err, "failed to parse FCGI response")
		}
	}

	return result, header, nil
}

func statusFromHeaders(h http.Header) (int, error) {
	text := h.Get("Status")

	h.Del("Status")

	if text == "" {
		return 200, nil
	}

	return strconv.Atoi(text[0:3])
}

func chunked(te []string) bool { return len(te) > 0 && te[0] == "chunked" }

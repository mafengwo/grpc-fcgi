package fcgi

import (
	"fmt"
	"net/http"
	"strings"
)

// ParamsFromRequest provides a standard way to convert an http.Request to
// the key-value pairs that are passed as FCGI parameters.
func ParamsFromRequest(r *http.Request) map[string][]string {
	params := map[string][]string{
		"REQUEST_METHOD":  {r.Method},
		"SERVER_PROTOCOL": {fmt.Sprintf("HTTP/%d.%d", r.ProtoMajor, r.ProtoMinor)},
		"HTTP_HOST":       {r.Host},
		"CONTENT_LENGTH":  {fmt.Sprintf("%d", r.ContentLength)},
		"CONTENT_TYPE":    {r.Header.Get("Content-Type")},
		"REQUEST_URI":     {r.RequestURI},
		"PATH_INFO":       {r.URL.Path},
	}

	for key, vals := range r.Header {
		name := fmt.Sprintf("HTTP_%s",
			strings.ToUpper(strings.Replace(key, "-", "_", -1)))
		params[name] = vals
	}

	https := "Off"
	if r.TLS != nil && r.TLS.HandshakeComplete {
		https = "On"
	}
	params["HTTPS"] = []string{https}

	// TODO(knorton): REMOTE_HOST and REMOTE_PORT

	return params
}

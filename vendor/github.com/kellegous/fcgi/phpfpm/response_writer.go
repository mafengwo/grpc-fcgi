package phpfpm

import (
	"bytes"
	"net/http"
)

type responseWriter struct {
	Status int
	Head   http.Header
	Body   bytes.Buffer
}

func (w *responseWriter) Header() http.Header {
	return w.Head
}

func (w *responseWriter) Write(b []byte) (int, error) {
	return w.Body.Write(b)
}

func (w *responseWriter) WriteHeader(status int) {
	w.Status = status
}

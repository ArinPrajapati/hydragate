package plugin

import (
	"bytes"
	"net/http"
)

type ResponseCapture struct {
	http.ResponseWriter
	StatusCode  int
	Body        bytes.Buffer
	headers     http.Header
	wroteHeader bool
}

func NewResponseCapture(w http.ResponseWriter) *ResponseCapture {
	return &ResponseCapture{
		ResponseWriter: w,
		StatusCode:     200,
		headers:        make(http.Header),
	}
}

func (r *ResponseCapture) Header() http.Header {
	return r.headers
}

func (r *ResponseCapture) WriteHeader(code int) {
	if !r.wroteHeader {
		r.StatusCode = code
		r.wroteHeader = true
	}
}

func (r *ResponseCapture) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(200)
	}
	return r.Body.Write(b)
}

func (r *ResponseCapture) Flush() {
	for k, vv := range r.headers {
		for _, v := range vv {
			r.ResponseWriter.Header().Add(k, v)
		}
	}
	r.ResponseWriter.WriteHeader(r.StatusCode)
	r.ResponseWriter.Write(r.Body.Bytes())
}

func (r *ResponseCapture) Bytes() []byte {
	return r.Body.Bytes()
}

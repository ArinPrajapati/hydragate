package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid" // id generator
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func Logger(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		start := time.Now()
		id := uuid.New().String()
		sr := &statusRecorder{
			ResponseWriter: res,
			statusCode:     http.StatusOK,
		}
		fn.ServeHTTP(sr, req)

		fmt.Printf("[%d] %s %v %s %s\n", sr.statusCode, req.Method, time.Since(start), id, req.URL.Path)
	})
}

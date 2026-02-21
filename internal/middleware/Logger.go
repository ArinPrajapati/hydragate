package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
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

		slog.Info("request completed",
			"status", sr.statusCode,
			"method", req.Method,
			"path", req.URL.Path,
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", id,
			"remote_addr", req.RemoteAddr,
		)
	})
}

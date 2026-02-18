package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid" // id generator
)

func Logger(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		start := time.Now()
		id := uuid.New().String()

		fmt.Printf("%s %v %s %s\n", req.Method, start.Format(time.RFC3339), id, req.URL.Path)
		fn.ServeHTTP(res, req)
		fmt.Printf("%s %v %s %s\n", req.Method, time.Since(start), id, req.URL.Path)
	})
}

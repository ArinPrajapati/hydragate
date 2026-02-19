package middleware

import "net/http"

func Chain(fn http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for _, m := range middlewares {
		fn = m(fn)
	}
	return fn
}

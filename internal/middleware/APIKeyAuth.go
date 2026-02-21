package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"hydragate/internal/urlpath"
)

type APIKeyAuthConfig struct {
	Keys            map[string]string // api_key → label
	ProtectedRoutes map[string]bool
}

func APIKeyAuth(cfg APIKeyAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("APIKeyAuth")
			parsed, err := urlpath.Parse(r.URL.Path)
			if err != nil {
				writeAuthError(w, http.StatusBadRequest, "invalid request path")
				return
			}

			if !cfg.ProtectedRoutes[parsed.Prefix] {
				next.ServeHTTP(w, r)
				return
			}

			apiKey := r.Header.Get("X-API-Key")

			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			label, valid := cfg.Keys[apiKey]
			if !valid {
				slog.Warn("invalid api key",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				writeAuthError(w, http.StatusUnauthorized, "invalid api key")
				return
			}

			r.Header.Set("X-Authenticated-By", "api-key")
			r.Header.Set("X-API-Key-Name", label)

			slog.Debug("api key auth passed",
				"path", r.URL.Path,
				"key_name", label,
			)

			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"hydragate/internal/auth"
	"hydragate/internal/urlpath"
)

type JWTAuthConfig struct {
	Secret          string
	ForwardClaims   map[string]string
	ProtectedRoutes map[string]bool
}

func JWTAuth(cfg JWTAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parsed, err := urlpath.Parse(r.URL.Path)
			if err != nil {
				writeAuthError(w, http.StatusBadRequest, "invalid request path")
				return
			}

			if !cfg.ProtectedRoutes[parsed.Prefix] {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeAuthError(w, http.StatusUnauthorized, "invalid authorization format, expected: Bearer <token>")
				return
			}

			tokenString := parts[1]

			claims, err := auth.ValidateToken(tokenString, cfg.Secret)
			if err != nil {
				slog.Warn("jwt validation failed",
					"error", err.Error(),
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			for claimName, headerName := range cfg.ForwardClaims {
				if value, ok := claims[claimName]; ok {
					r.Header.Set(headerName, fmt.Sprintf("%v", value))
				}
			}

			slog.Debug("jwt auth passed",
				"path", r.URL.Path,
				"claims_forwarded", len(cfg.ForwardClaims),
			)

			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

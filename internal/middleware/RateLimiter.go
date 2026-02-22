package middleware

import (
	"context"
	_ "embed"
	"log/slog"
	"net"
	"net/http"
	"time"

	"hydragate/internal/app"

	"github.com/redis/go-redis/v9"
)

//go:embed rate_limit.lua
var rateLimitLua string

var tokenBucketScript = redis.NewScript(rateLimitLua)

func RateLimiter(rdb *redis.Client, cfg app.RateLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			}

			key := "rate_limit:ip:" + ip
			now := time.Now().Unix()

			allowed, err := tokenBucketScript.Run(context.Background(), rdb, []string{key}, cfg.Capacity, cfg.RefillRate, now, 1).Result()
			if err != nil {
				slog.Error("RateLimiter Redis script error", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			if allowed.(int64) == 0 {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"context"
	_ "embed"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"hydragate/internal/app"

	"github.com/redis/go-redis/v9"
)

//go:embed rate_limit.lua
var rateLimitLua string

var tokenBucketScript = redis.NewScript(rateLimitLua)

var (
	rateLimitWarnTime sync.Mutex
	rateLimitLastWarn time.Time
)

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
				logRateLimitWarning(ip, r.URL.Path, r.Method, cfg.Capacity, cfg.RefillRate)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			slog.Debug("rate limit check passed",
				"ip", ip,
				"path", r.URL.Path,
				"method", r.Method,
				"tokens_remaining", allowed.(int64),
			)
			next.ServeHTTP(w, r)
		})
	}
}

// logRateLimitWarning logs a rate limit warning every 10 seconds to prevent log spam
func logRateLimitWarning(ip, path, method string, capacity, refillRate int) {
	rateLimitWarnTime.Lock()
	defer rateLimitWarnTime.Unlock()

	now := time.Now()
	if now.Sub(rateLimitLastWarn) >= 10*time.Second {
		slog.Warn("rate limit exceeded",
			"ip", ip,
			"path", path,
			"method", method,
			"capacity", capacity,
			"refill_rate", refillRate,
		)
		rateLimitLastWarn = now
	}
}

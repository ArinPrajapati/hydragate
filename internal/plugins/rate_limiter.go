package plugins

import (
	"context"
	_ "embed"
	"log/slog"
	"net"
	"net/http"
	"time"

	"hydragate/internal/plugin"

	"github.com/redis/go-redis/v9"
)

//go:embed rate_limit.lua
var rateLimitLua string

var tokenBucketScript = redis.NewScript(rateLimitLua)

// RateLimiterPlugin enforces a per-IP token-bucket rate limit using Redis.
// It runs in the PhasePreRoute so aborted requests never reach the proxy.
type RateLimiterPlugin struct {
	BasePlugin
	rdb      *redis.Client
	capacity int
	refill   int
}

func (p *RateLimiterPlugin) Name() string { return "rate_limiter" }

// RateLimiterFactory is the PluginFactory for RateLimiterPlugin.
// The Redis client cannot come from the config map (it's a live connection),
// so we use NewRateLimiterFactory to close over it.
func NewRateLimiterFactory(rdb *redis.Client) plugin.PluginFactory {
	return func(cfg map[string]interface{}, logger *slog.Logger) (plugin.Plugin, error) {
		p := &RateLimiterPlugin{rdb: rdb}
		p.Logger = logger
		p.Config = cfg

		if cap, ok := cfg["capacity"].(float64); ok {
			p.capacity = int(cap)
		}
		if refill, ok := cfg["refill_rate"].(float64); ok {
			p.refill = int(refill)
		}

		return p, nil
	}
}

// OnPreRoute runs before route matching. If the client exceeds the rate limit
// the request is aborted with 429 and never forwarded to the backend.
func (p *RateLimiterPlugin) OnPreRoute(ctx *plugin.PluginContext) error {
	if p.capacity <= 0 || p.refill <= 0 {
		// Not configured — pass through.
		return nil
	}

	ip := clientIP(ctx.Request)
	key := "rate_limit:ip:" + ip
	now := time.Now().Unix()

	result, err := tokenBucketScript.Run(
		context.Background(), p.rdb,
		[]string{key},
		p.capacity, p.refill, now, 1,
	).Result()

	if err != nil {
		p.Logger.Error("rate limiter Redis script error", "error", err)
		// Fail open: let the request through rather than block everyone on Redis issues.
		return nil
	}

	if result.(int64) == 0 {
		p.Logger.Warn("rate limit exceeded",
			"ip", ip,
			"path", ctx.Request.URL.Path,
			"method", ctx.Request.Method,
			"capacity", p.capacity,
			"refill_rate", p.refill,
			"request_id", ctx.Metadata["request_id"],
		)
		ctx.Abort = true
		ctx.AbortCode = http.StatusTooManyRequests
		ctx.AbortBody = []byte(`{"error":"rate limit exceeded"}`)
	}

	return nil
}

func (p *RateLimiterPlugin) Metrics() map[string]float64 {
	return map[string]float64{
		"hydragate_plugin_rate_limiter_capacity":    float64(p.capacity),
		"hydragate_plugin_rate_limiter_refill_rate": float64(p.refill),
	}
}

// clientIP extracts the real client IP, honouring X-Forwarded-For.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

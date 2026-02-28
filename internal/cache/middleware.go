package cache

import (
	"bytes"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"hydragate/internal/app"
	"hydragate/internal/urlpath"

	"github.com/redis/go-redis/v9"
)

var (
	// Redis health check state
	redisLastWarnTime sync.Mutex
	redisLastWarn     time.Time
)

// Cache creates a caching middleware
func Cache(rdb *redis.Client, config *app.GatewayConfig, getRoute func(string) (app.RouteConfig, bool)) func(http.Handler) http.Handler {
	cacheClient := NewRedisCache(rdb)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Check Redis health
			if !cacheClient.IsHealthy(ctx) {
				logRedisWarning("cache Redis is down, skipping cache")
				next.ServeHTTP(w, r)
				return
			}

			// Parse path to get route prefix
			parsed, err := urlpath.Parse(r.URL.Path)
			if err != nil {
				slog.Debug("failed to parse path", "path", r.URL.Path, "error", err)
				next.ServeHTTP(w, r)
				return
			}

			// Get route config
			route, found := getRoute(parsed.Prefix)
			if !found {
				// No route found, skip cache
				next.ServeHTTP(w, r)
				return
			}

			// Resolve cache config for this request
			cacheConfig := ResolveCacheConfig(r.URL.Path, &route, &config.Cache)

			// Check if request is cacheable
			if !isRequestCacheable(r, &cacheConfig) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract user identity for protected routes
			userIdentity := extractUserIdentity(r, route.Protected)

			// Generate cache key (includes user identity for protected routes)
			cacheKey := GenerateCacheKey(r, &cacheConfig, route.Route, userIdentity)

			// Try to get from cache
			entry, err := cacheClient.Get(ctx, cacheKey)
			if err != nil {
				slog.Error("cache get error", "key", cacheKey, "error", err)
				// Continue to backend on error
				next.ServeHTTP(w, r)
				return
			}

			// Cache hit - serve from cache
			if entry != nil && entry.IsFresh() {
				serveFromCache(w, entry)
				slog.Info("cache hit", "key", cacheKey, "path", r.URL.Path, "method", r.Method, "user", userIdentity)
				return
			}

			// Cache miss - proxy to backend and cache response
			cacheMissHandler(w, r, next, &cacheConfig, cacheKey, cacheClient, userIdentity)
		})
	}
}

// extractUserIdentity extracts user identity from request headers for protected routes
// Returns empty string for unprotected routes or when identity cannot be determined
func extractUserIdentity(r *http.Request, isProtected bool) string {
	if !isProtected {
		return ""
	}

	// Check for JWT-based authentication (claims forwarded to headers)
	// Try common claim headers: X-User-Id, X-User-Email, X-User-Sub
	for _, headerName := range []string{"X-User-Id", "X-User-Sub", "X-User-Email"} {
		if userId := r.Header.Get(headerName); userId != "" {
			return userId
		}
	}

	// Check for API key authentication
	if apiKeyName := r.Header.Get("X-API-Key-Name"); apiKeyName != "" {
		return "apikey:" + apiKeyName
	}

	// No identity found - use anonymous (this will still prevent cache poisoning by
	// ensuring all unauthenticated requests share the same cache key)
	return "anonymous"
}

// isRequestCacheable checks if the HTTP request is cacheable
func isRequestCacheable(r *http.Request, config *ResolvedCacheConfig) bool {
	// Check if caching is enabled
	if !config.Enabled {
		return false
	}

	// Check HTTP method
	methodCacheable := slices.Contains(config.Methods, r.Method)
	if !methodCacheable {
		return false
	}

	return true
}

// cacheMissHandler handles cache misses - proxies to backend and caches response
func cacheMissHandler(
	w http.ResponseWriter,
	r *http.Request,
	next http.Handler,
	config *ResolvedCacheConfig,
	cacheKey string,
	cacheClient *RedisCache,
	userIdentity string,
) {
	// Create response recorder to capture response
	recorder := &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           bytes.NewBuffer(nil),
	}

	// Call next handler (proxy to backend)
	next.ServeHTTP(recorder, r)

	// Check if response is cacheable
	if !IsCacheable(config, recorder.statusCode, recorder.Header(), recorder.size) {
		slog.Debug("response not cacheable", "key", cacheKey, "status", recorder.statusCode)
		return
	}

	// Create cache entry
	entry := NewCacheEntry(
		recorder.statusCode,
		recorder.Header(),
		recorder.body.Bytes(),
		config.TTL,
	)

	// Store in cache
	if err := cacheClient.Set(r.Context(), cacheKey, entry); err != nil {
		slog.Error("cache set error", "key", cacheKey, "error", err)
		return
	}

	slog.Info("cache stored", "key", cacheKey, "path", r.URL.Path, "ttl", config.TTL, "user", userIdentity)
}

// serveFromCache serves a cached response
func serveFromCache(w http.ResponseWriter, entry *CacheEntry) {
	// Set headers
	// Handle multi-value headers that were concatenated with ", " separator
	for key, value := range entry.Headers {
		// Split multi-value headers back
		values := strings.Split(value, ", ")
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	// Set status code
	w.WriteHeader(entry.StatusCode)

	// Write body
	w.Write(entry.Body)
}

// logRedisWarning logs a Redis warning every 10 seconds
func logRedisWarning(message string) {
	redisLastWarnTime.Lock()
	defer redisLastWarnTime.Unlock()

	now := time.Now()
	if now.Sub(redisLastWarn) >= 10*time.Second {
		slog.Warn(message)
		redisLastWarn = now
	}
}

// responseRecorder captures the HTTP response
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	size       int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.body.Write(b)
	if err != nil {
		return n, err
	}
	r.size += n
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

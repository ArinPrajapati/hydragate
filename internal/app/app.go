package app

import "hydragate/internal/plugin"

type TransformConfig struct {
	AddHeaders    map[string]string `json:"add_headers,omitempty"`
	RemoveHeaders []string          `json:"remove_headers,omitempty"`
	PathRewrite   string            `json:"path_rewrite,omitempty"`
}

// Cache Configuration
type GlobalCacheConfig struct {
	Enabled     bool           `json:"enabled"`
	DefaultTTL  int            `json:"default_ttl"`
	Methods     []string       `json:"methods"`
	StatusCodes []int          `json:"status_codes"`
	Key         CacheKeyConfig `json:"key"`
	MaxSize     int            `json:"max_size"`
}

type CacheKeyConfig struct {
	IncludeQuery        bool     `json:"include_query"`
	IncludeHeaders      []string `json:"include_headers"`
	RespectCacheControl bool     `json:"respect_cache_control"`
	ETagValidation      bool     `json:"etag_validation"`
}

type RouteCacheConfig struct {
	Enabled     *bool           `json:"enabled,omitempty"`
	TTL         int             `json:"ttl,omitempty"`
	Methods     []string        `json:"methods,omitempty"`
	StatusCodes []int           `json:"status_codes,omitempty"`
	Key         *CacheKeyConfig `json:"key,omitempty"`
}

type CachePathOverride struct {
	Path        string          `json:"path"`
	Enabled     *bool           `json:"enabled,omitempty"`
	TTL         int             `json:"ttl,omitempty"`
	Methods     []string        `json:"methods,omitempty"`
	StatusCodes []int           `json:"status_codes,omitempty"`
	Key         *CacheKeyConfig `json:"key,omitempty"`
}

// Route Configuration (updated)
type RouteConfig struct {
	Route      string              `json:"route"`
	Target     string              `json:"target"`
	Protected  bool                `json:"protected"`
	Transform  TransformConfig     `json:"transform"`
	Cache      *RouteCacheConfig   `json:"cache,omitempty"`
	CachePaths []CachePathOverride `json:"cache_paths,omitempty"`
}

type RateLimitConfig struct {
	Enabled    bool `json:"enabled"`
	Capacity   int  `json:"capacity"`
	RefillRate int  `json:"refill_rate"`
}

// Gateway Configuration (updated)
type GatewayConfig struct {
	JWTSecret     string               `json:"jwt_secret"`
	ForwardClaims map[string]string    `json:"forward_claims"`
	APIKeys       map[string]string    `json:"api_keys"`
	RateLimit     RateLimitConfig      `json:"rate_limit"`
	Cache         GlobalCacheConfig    `json:"cache"`
	Routes        []RouteConfig        `json:"routes"`
	Plugins       plugin.PluginsConfig `json:"plugins"`
}

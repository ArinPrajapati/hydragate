package cache

import (
	"strings"

	"hydragate/internal/app"
)

// ResolvedCacheConfig - fully resolved configuration after applying 3-level precedence
type ResolvedCacheConfig struct {
	Enabled             bool
	TTL                 int
	Methods             []string
	StatusCodes         []int
	IncludeQuery        bool
	IncludeHeaders      []string
	RespectCacheControl bool
	ETagValidation      bool
	MaxSize             int
}

// ResolveCacheConfig resolves cache config using 3-level precedence
// Precedence: Path Override > Route Config > Global Default
func ResolveCacheConfig(
	fullPath string, // Full request path (e.g., "/api/products")
	route *app.RouteConfig,
	global *app.GlobalCacheConfig,
) ResolvedCacheConfig {

	// Level 1: Start with global defaults
	config := ResolvedCacheConfig{
		Enabled:             global.Enabled,
		TTL:                 global.DefaultTTL,
		Methods:             global.Methods,
		StatusCodes:         global.StatusCodes,
		IncludeQuery:        global.Key.IncludeQuery,
		IncludeHeaders:      global.Key.IncludeHeaders,
		RespectCacheControl: global.Key.RespectCacheControl,
		ETagValidation:      global.Key.ETagValidation,
		MaxSize:             global.MaxSize,
	}

	// Level 2: Apply route-level overrides if exists
	if route.Cache != nil {
		applyRouteCacheConfig(&config, route.Cache)
	}

	// Level 3: Apply path-specific overrides if exists (highest precedence)
	if len(route.CachePaths) > 0 {
		pathWithoutPrefix := getPathWithoutPrefix(fullPath, route.Route)
		if pathOverride := findPathOverride(pathWithoutPrefix, route.CachePaths); pathOverride != nil {
			applyPathCacheConfig(&config, pathOverride)
		}
	}

	return config
}

// applyRouteCacheConfig applies route-level overrides
func applyRouteCacheConfig(config *ResolvedCacheConfig, routeCache *app.RouteCacheConfig) {
	if routeCache.Enabled != nil {
		config.Enabled = *routeCache.Enabled
	}
	if routeCache.TTL > 0 {
		config.TTL = routeCache.TTL
	}
	if len(routeCache.Methods) > 0 {
		config.Methods = routeCache.Methods
	}
	if len(routeCache.StatusCodes) > 0 {
		config.StatusCodes = routeCache.StatusCodes
	}
	if routeCache.Key != nil {
		if routeCache.Key.IncludeQuery {
			config.IncludeQuery = routeCache.Key.IncludeQuery
		}
		if len(routeCache.Key.IncludeHeaders) > 0 {
			config.IncludeHeaders = routeCache.Key.IncludeHeaders
		}
		if routeCache.Key.RespectCacheControl {
			config.RespectCacheControl = routeCache.Key.RespectCacheControl
		}
		if routeCache.Key.ETagValidation {
			config.ETagValidation = routeCache.Key.ETagValidation
		}
	}
}

// applyPathCacheConfig applies path-level overrides
func applyPathCacheConfig(config *ResolvedCacheConfig, pathOverride *app.CachePathOverride) {
	if pathOverride.Enabled != nil {
		config.Enabled = *pathOverride.Enabled
	}
	if pathOverride.TTL > 0 {
		config.TTL = pathOverride.TTL
	}
	if len(pathOverride.Methods) > 0 {
		config.Methods = pathOverride.Methods
	}
	if len(pathOverride.StatusCodes) > 0 {
		config.StatusCodes = pathOverride.StatusCodes
	}
	if pathOverride.Key != nil {
		if pathOverride.Key.IncludeQuery {
			config.IncludeQuery = pathOverride.Key.IncludeQuery
		}
		config.IncludeHeaders = pathOverride.Key.IncludeHeaders
		if pathOverride.Key.RespectCacheControl {
			config.RespectCacheControl = pathOverride.Key.RespectCacheControl
		}
		if pathOverride.Key.ETagValidation {
			config.ETagValidation = pathOverride.Key.ETagValidation
		}
	}
}

// findPathOverride finds the matching path override
// Supports exact match and prefix match (if path ends with /*)
// Exact matches take precedence over wildcard matches
func findPathOverride(path string, overrides []app.CachePathOverride) *app.CachePathOverride {
	// First pass: look for exact match
	for i := range overrides {
		overridePath := overrides[i].Path
		if !strings.HasSuffix(overridePath, "/*") && overridePath == path {
			return &overrides[i]
		}
	}

	// Second pass: look for wildcard match
	for i := range overrides {
		overridePath := overrides[i].Path
		if strings.HasSuffix(overridePath, "/*") {
			prefix := strings.TrimSuffix(overridePath, "/*")
			// Match if path starts with prefix (including exact match to prefix)
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return &overrides[i]
			}
		}
	}

	return nil
}

// getPathWithoutPrefix extracts the path without the route prefix
func getPathWithoutPrefix(fullPath, routePrefix string) string {
	// Remove leading slash from full path and route prefix
	path := strings.TrimPrefix(fullPath, "/")
	prefix := strings.TrimPrefix(routePrefix, "/")

	// Remove route prefix from path
	path = strings.TrimPrefix(path, prefix)

	// Normalize to start with /
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

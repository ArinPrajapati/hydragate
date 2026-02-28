package cache

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// GenerateCacheKey generates a human-readable cache key
// Format: gateway:cache:{method}:{path_prefix}:{path}:{query}:{user_identity}
// Example: gateway:cache:GET:api:products:id:123
func GenerateCacheKey(r *http.Request, config *ResolvedCacheConfig, routePrefix string, userIdentity string) string {
	// Parse and normalize the path
	path := normalizePath(r.URL.Path)

	// Remove route prefix
	pathWithoutPrefix := removeRoutePrefix(path, routePrefix)

	// Split path into parts
	pathParts := strings.Split(strings.Trim(pathWithoutPrefix, "/"), "/")

	// Build cache key parts
	keyParts := []string{
		"gateway",
		"cache",
		r.Method,
		routePrefix,
	}

	// Add path parts
	for _, part := range pathParts {
		if part != "" {
			keyParts = append(keyParts, part)
		}
	}

	// Add query params if configured
	if config.IncludeQuery && r.URL.RawQuery != "" {
		normalizedQuery := normalizeQuery(r.URL.RawQuery)
		keyParts = append(keyParts, normalizedQuery)
	}

	// Add configured headers to cache key
	if len(config.IncludeHeaders) > 0 {
		headers := normalizeHeaders(r, config.IncludeHeaders)
		if headers != "" {
			keyParts = append(keyParts, headers)
		}
	}

	// Add user identity if present (for protected routes)
	if userIdentity != "" {
		keyParts = append(keyParts, userIdentity)
	}

	// Join with colons for human-readable format
	return strings.Join(keyParts, ":")
}

// normalizePath normalizes the path by removing trailing slashes and ensuring consistent format
func normalizePath(path string) string {
	// Remove trailing slashes
	path = strings.TrimRight(path, "/")

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

// removeRoutePrefix removes the route prefix from the path
func removeRoutePrefix(path, routePrefix string) string {
	// Normalize path and prefix
	path = strings.TrimPrefix(path, "/")
	prefix := strings.TrimPrefix(routePrefix, "/")

	// Remove prefix from path
	path = strings.TrimPrefix(path, prefix)

	// Normalize
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

// normalizeQuery normalizes query parameters to a consistent format
func normalizeQuery(rawQuery string) string {
	// Parse query string
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery // Return raw if parsing fails
	}

	// Sort parameters for consistent key generation
	var params []string
	for key, vals := range values {
		for _, val := range vals {
			params = append(params, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// Sort parameters alphabetically for consistent cache keys
	sort.Strings(params)

	return strings.Join(params, "&")
}

// normalizeHeaders normalizes the specified headers for cache key
func normalizeHeaders(r *http.Request, headerNames []string) string {
	var headerParts []string

	for _, headerName := range headerNames {
		// Case-insensitive header lookup
		value := r.Header.Get(headerName)
		if value != "" {
			// Normalize header name to lowercase
			normalizedHeader := strings.ToLower(strings.TrimSpace(headerName))
			// Remove newlines and extra spaces from value
			normalizedValue := strings.Join(strings.Fields(value), " ")
			headerParts = append(headerParts, fmt.Sprintf("%s:%s", normalizedHeader, normalizedValue))
		}
	}

	if len(headerParts) == 0 {
		return ""
	}

	return strings.Join(headerParts, "|")
}

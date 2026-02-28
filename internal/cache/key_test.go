package cache

import (
	"net/http"
	"net/url"
	"testing"
)

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		routePrefix   string
		query         string
		config        *ResolvedCacheConfig
		headers       http.Header
		userIdentity  string
		expectedParts []string
	}{
		{
			name:        "simple GET request",
			method:      "GET",
			path:        "/api/products",
			routePrefix: "api",
			query:       "",
			config: &ResolvedCacheConfig{
				IncludeQuery:   false,
				IncludeHeaders: []string{},
			},
			headers:       http.Header{},
			userIdentity:  "",
			expectedParts: []string{"gateway", "cache", "GET", "api", "products"},
		},
		{
			name:        "GET request with user identity",
			method:      "GET",
			path:        "/api/products",
			routePrefix: "api",
			query:       "",
			config: &ResolvedCacheConfig{
				IncludeQuery:   false,
				IncludeHeaders: []string{},
			},
			headers:       http.Header{},
			userIdentity:  "user123",
			expectedParts: []string{"gateway", "cache", "GET", "api", "products", "user123"},
		},
		{
			name:        "GET request with query params",
			method:      "GET",
			path:        "/api/products",
			routePrefix: "api",
			query:       "page=2&limit=10",
			config: &ResolvedCacheConfig{
				IncludeQuery:   true,
				IncludeHeaders: []string{},
			},
			headers:       http.Header{},
			userIdentity:  "",
			expectedParts: []string{"gateway", "cache", "GET", "api", "products", "limit=10&page=2"},
		},
		{
			name:        "GET request with custom headers",
			method:      "GET",
			path:        "/api/products",
			routePrefix: "api",
			query:       "",
			config: &ResolvedCacheConfig{
				IncludeQuery:   false,
				IncludeHeaders: []string{"Accept", "Accept-Language"},
			},
			headers: http.Header{
				"Accept":          []string{"application/json"},
				"Accept-Language": []string{"en-US"},
			},
			userIdentity:  "",
			expectedParts: []string{"gateway", "cache", "GET", "api", "products", "accept:application/json|accept-language:en-US"},
		},
		{
			name:        "POST request",
			method:      "POST",
			path:        "/api/products",
			routePrefix: "api",
			query:       "",
			config: &ResolvedCacheConfig{
				IncludeQuery:   false,
				IncludeHeaders: []string{},
			},
			headers:       http.Header{},
			userIdentity:  "",
			expectedParts: []string{"gateway", "cache", "POST", "api", "products"},
		},
		{
			name:        "nested path",
			method:      "GET",
			path:        "/api/products/123/reviews",
			routePrefix: "api",
			query:       "",
			config: &ResolvedCacheConfig{
				IncludeQuery:   false,
				IncludeHeaders: []string{},
			},
			headers:       http.Header{},
			userIdentity:  "",
			expectedParts: []string{"gateway", "cache", "GET", "api", "products", "123", "reviews"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Method: tt.method,
				URL: &url.URL{
					Path:     tt.path,
					RawQuery: tt.query,
				},
				Header: tt.headers,
			}

			key := GenerateCacheKey(req, tt.config, tt.routePrefix, tt.userIdentity)

			expected := joinWithColons(tt.expectedParts)
			if key != expected {
				t.Errorf("GenerateCacheKey() = %v, want %v", key, expected)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/products", "/api/products"},
		{"/api/products/", "/api/products"},
		{"api/products", "/api/products"},
		{"/api/products//", "/api/products"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizePath(tt.input)
			if got != tt.expected {
				t.Errorf("normalizePath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRemoveRoutePrefix(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		routePrefix string
		expected    string
	}{
		{
			name:        "simple prefix removal",
			path:        "/api/products",
			routePrefix: "api",
			expected:    "/products",
		},
		{
			name:        "path equals prefix",
			path:        "/api",
			routePrefix: "api",
			expected:    "/",
		},
		{
			name:        "nested path",
			path:        "/api/v1/users",
			routePrefix: "api",
			expected:    "/v1/users",
		},
		{
			name:        "no leading slash",
			path:        "api/products",
			routePrefix: "api",
			expected:    "/products",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeRoutePrefix(tt.path, tt.routePrefix)
			if got != tt.expected {
				t.Errorf("removeRoutePrefix() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "sorted query params",
			query:    "limit=10&page=2",
			expected: "limit=10&page=2",
		},
		{
			name:     "unsorted query params",
			query:    "page=2&limit=10",
			expected: "limit=10&page=2",
		},
		{
			name:     "multiple params same key",
			query:    "tag=a&tag=b&tag=c",
			expected: "tag=a&tag=b&tag=c",
		},
		{
			name:     "complex query",
			query:    "sort=desc&limit=10&page=2&filter=active",
			expected: "filter=active&limit=10&page=2&sort=desc",
		},
		{
			name:     "empty query",
			query:    "",
			expected: "",
		},
		{
			name:     "invalid query (return as-is)",
			query:    "invalid&query",
			expected: "invalid=&query=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeQuery(tt.query)
			if got != tt.expected {
				t.Errorf("normalizeQuery() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNormalizeHeaders(t *testing.T) {
	tests := []struct {
		name        string
		headers     http.Header
		headerNames []string
		expected    string
	}{
		{
			name: "single header",
			headers: http.Header{
				"Accept": []string{"application/json"},
			},
			headerNames: []string{"Accept"},
			expected:    "accept:application/json",
		},
		{
			name: "multiple headers",
			headers: http.Header{
				"Accept":          []string{"application/json"},
				"Accept-Language": []string{"en-US"},
			},
			headerNames: []string{"Accept", "Accept-Language"},
			expected:    "accept:application/json|accept-language:en-US",
		},
		{
			name: "header with extra spaces",
			headers: http.Header{
				"Accept": []string{"  application/json  "},
			},
			headerNames: []string{"Accept"},
			expected:    "accept:application/json",
		},
		{
			name:        "missing header",
			headers:     http.Header{},
			headerNames: []string{"Accept"},
			expected:    "",
		},
		{
			name: "mixed case header name",
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			headerNames: []string{"content-type"},
			expected:    "content-type:application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: tt.headers,
			}

			got := normalizeHeaders(req, tt.headerNames)
			if got != tt.expected {
				t.Errorf("normalizeHeaders() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractUserIdentity(t *testing.T) {
	tests := []struct {
		name        string
		headers     http.Header
		isProtected bool
		expected    string
	}{
		{
			name:        "unprotected route",
			headers:     http.Header{},
			isProtected: false,
			expected:    "",
		},
		{
			name: "JWT user-id",
			headers: http.Header{
				"X-User-Id": []string{"user123"},
			},
			isProtected: true,
			expected:    "user123",
		},
		{
			name: "JWT sub claim",
			headers: http.Header{
				"X-User-Sub": []string{"sub456"},
			},
			isProtected: true,
			expected:    "sub456",
		},
		{
			name: "API key authentication",
			headers: http.Header{
				"X-Api-Key-Name": []string{"client-app"},
			},
			isProtected: true,
			expected:    "apikey:client-app",
		},
		{
			name:        "no identity (anonymous)",
			headers:     http.Header{},
			isProtected: true,
			expected:    "anonymous",
		},
		{
			name: "JWT takes precedence over API key",
			headers: http.Header{
				"X-User-Id":      []string{"user123"},
				"X-Api-Key-Name": []string{"client-app"},
			},
			isProtected: true,
			expected:    "user123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: tt.headers,
			}

			got := extractUserIdentity(req, tt.isProtected)
			if got != tt.expected {
				t.Errorf("extractUserIdentity() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func joinWithColons(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ":"
		}
		result += part
	}
	return result
}

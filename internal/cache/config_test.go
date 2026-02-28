package cache

import (
	"hydragate/internal/app"
	"testing"
)

func TestResolveCacheConfig(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		route    *app.RouteConfig
		global   *app.GlobalCacheConfig
		expected ResolvedCacheConfig
	}{
		{
			name: "global defaults only",
			path: "/api/products",
			route: &app.RouteConfig{
				Route: "api",
				Cache: nil,
			},
			global: &app.GlobalCacheConfig{
				Enabled:     true,
				DefaultTTL:  300,
				Methods:     []string{"GET"},
				StatusCodes: []int{200},
				Key: app.CacheKeyConfig{
					IncludeQuery:        false,
					IncludeHeaders:      []string{},
					RespectCacheControl: true,
					ETagValidation:      false,
				},
				MaxSize: 1024 * 1024,
			},
			expected: ResolvedCacheConfig{
				Enabled:             true,
				TTL:                 300,
				Methods:             []string{"GET"},
				StatusCodes:         []int{200},
				IncludeQuery:        false,
				IncludeHeaders:      []string{},
				RespectCacheControl: true,
				ETagValidation:      false,
				MaxSize:             1024 * 1024,
			},
		},
		{
			name: "route-level override",
			path: "/api/products",
			route: &app.RouteConfig{
				Route: "api",
				Cache: &app.RouteCacheConfig{
					Enabled: boolPtr(false),
					TTL:     600,
					Methods: []string{"GET", "HEAD"},
				},
			},
			global: &app.GlobalCacheConfig{
				Enabled:     true,
				DefaultTTL:  300,
				Methods:     []string{"GET"},
				StatusCodes: []int{200},
				Key: app.CacheKeyConfig{
					IncludeQuery: false,
				},
				MaxSize: 1024 * 1024,
			},
			expected: ResolvedCacheConfig{
				Enabled:             false,
				TTL:                 600,
				Methods:             []string{"GET", "HEAD"},
				StatusCodes:         []int{200},
				IncludeQuery:        false,
				IncludeHeaders:      []string{},
				RespectCacheControl: false,
				ETagValidation:      false,
				MaxSize:             1024 * 1024,
			},
		},
		{
			name: "path-level override",
			path: "/api/products/special",
			route: &app.RouteConfig{
				Route: "api",
				Cache: &app.RouteCacheConfig{
					Enabled: boolPtr(true),
					TTL:     300,
				},
				CachePaths: []app.CachePathOverride{
					{
						Path:    "/products/special",
						TTL:     900,
						Enabled: boolPtr(true),
					},
				},
			},
			global: &app.GlobalCacheConfig{
				Enabled:     true,
				DefaultTTL:  300,
				Methods:     []string{"GET"},
				StatusCodes: []int{200},
				Key: app.CacheKeyConfig{
					IncludeQuery: false,
				},
				MaxSize: 1024 * 1024,
			},
			expected: ResolvedCacheConfig{
				Enabled:             true,
				TTL:                 900,
				Methods:             []string{"GET"},
				StatusCodes:         []int{200},
				IncludeQuery:        false,
				IncludeHeaders:      []string{},
				RespectCacheControl: false,
				ETagValidation:      false,
				MaxSize:             1024 * 1024,
			},
		},
		{
			name: "wildcard path override",
			path: "/api/products/123",
			route: &app.RouteConfig{
				Route: "api",
				Cache: &app.RouteCacheConfig{
					Enabled: boolPtr(true),
					TTL:     300,
				},
				CachePaths: []app.CachePathOverride{
					{
						Path: "/products/*",
						TTL:  600,
					},
				},
			},
			global: &app.GlobalCacheConfig{
				Enabled:     true,
				DefaultTTL:  300,
				Methods:     []string{"GET"},
				StatusCodes: []int{200},
				Key: app.CacheKeyConfig{
					IncludeQuery: false,
				},
				MaxSize: 1024 * 1024,
			},
			expected: ResolvedCacheConfig{
				Enabled:             true,
				TTL:                 600,
				Methods:             []string{"GET"},
				StatusCodes:         []int{200},
				IncludeQuery:        false,
				IncludeHeaders:      []string{},
				RespectCacheControl: false,
				ETagValidation:      false,
				MaxSize:             1024 * 1024,
			},
		},
		{
			name: "exact path override with nested path",
			path: "/api/products",
			route: &app.RouteConfig{
				Route: "api",
				Cache: &app.RouteCacheConfig{
					Enabled: boolPtr(true),
					TTL:     300,
				},
				CachePaths: []app.CachePathOverride{
					{
						Path: "/products",
						TTL:  600,
					},
					{
						Path: "/products/*",
						TTL:  900,
					},
				},
			},
			global: &app.GlobalCacheConfig{
				Enabled:     true,
				DefaultTTL:  300,
				Methods:     []string{"GET"},
				StatusCodes: []int{200},
				Key: app.CacheKeyConfig{
					IncludeQuery: false,
				},
				MaxSize: 1024 * 1024,
			},
			expected: ResolvedCacheConfig{
				Enabled:             true,
				TTL:                 600,
				Methods:             []string{"GET"},
				StatusCodes:         []int{200},
				IncludeQuery:        false,
				IncludeHeaders:      []string{},
				RespectCacheControl: false,
				ETagValidation:      false,
				MaxSize:             1024 * 1024,
			},
		},
		{
			name: "path override with key configuration",
			path: "/api/products/123",
			route: &app.RouteConfig{
				Route: "api",
				Cache: &app.RouteCacheConfig{
					Enabled: boolPtr(true),
					TTL:     300,
				},
				CachePaths: []app.CachePathOverride{
					{
						Path: "/products/*",
						TTL:  600,
						Key: &app.CacheKeyConfig{
							IncludeQuery:   true,
							IncludeHeaders: []string{"Accept"},
						},
					},
				},
			},
			global: &app.GlobalCacheConfig{
				Enabled:     true,
				DefaultTTL:  300,
				Methods:     []string{"GET"},
				StatusCodes: []int{200},
				Key: app.CacheKeyConfig{
					IncludeQuery:        false,
					IncludeHeaders:      []string{},
					RespectCacheControl: true,
				},
				MaxSize: 1024 * 1024,
			},
			expected: ResolvedCacheConfig{
				Enabled:             true,
				TTL:                 600,
				Methods:             []string{"GET"},
				StatusCodes:         []int{200},
				IncludeQuery:        true,
				IncludeHeaders:      []string{"Accept"},
				RespectCacheControl: true,
				ETagValidation:      false,
				MaxSize:             1024 * 1024,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCacheConfig(tt.path, tt.route, tt.global)

			if got.Enabled != tt.expected.Enabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tt.expected.Enabled)
			}
			if got.TTL != tt.expected.TTL {
				t.Errorf("TTL = %v, want %v", got.TTL, tt.expected.TTL)
			}
			if !sliceEqual(got.Methods, tt.expected.Methods) {
				t.Errorf("Methods = %v, want %v", got.Methods, tt.expected.Methods)
			}
			if !sliceEqualInt(got.StatusCodes, tt.expected.StatusCodes) {
				t.Errorf("StatusCodes = %v, want %v", got.StatusCodes, tt.expected.StatusCodes)
			}
			if got.IncludeQuery != tt.expected.IncludeQuery {
				t.Errorf("IncludeQuery = %v, want %v", got.IncludeQuery, tt.expected.IncludeQuery)
			}
			if !sliceEqual(got.IncludeHeaders, tt.expected.IncludeHeaders) {
				t.Errorf("IncludeHeaders = %v, want %v", got.IncludeHeaders, tt.expected.IncludeHeaders)
			}
			if got.RespectCacheControl != tt.expected.RespectCacheControl {
				t.Errorf("RespectCacheControl = %v, want %v", got.RespectCacheControl, tt.expected.RespectCacheControl)
			}
			if got.ETagValidation != tt.expected.ETagValidation {
				t.Errorf("ETagValidation = %v, want %v", got.ETagValidation, tt.expected.ETagValidation)
			}
			if got.MaxSize != tt.expected.MaxSize {
				t.Errorf("MaxSize = %v, want %v", got.MaxSize, tt.expected.MaxSize)
			}
		})
	}
}

func TestFindPathOverride(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		overrides []app.CachePathOverride
		want      *app.CachePathOverride
	}{
		{
			name: "exact match",
			path: "/products",
			overrides: []app.CachePathOverride{
				{Path: "/products", TTL: 100},
				{Path: "/users", TTL: 200},
			},
			want: &app.CachePathOverride{Path: "/products", TTL: 100},
		},
		{
			name: "wildcard match",
			path: "/products/123",
			overrides: []app.CachePathOverride{
				{Path: "/products/*", TTL: 150},
				{Path: "/users", TTL: 200},
			},
			want: &app.CachePathOverride{Path: "/products/*", TTL: 150},
		},
		{
			name: "exact match takes precedence over wildcard",
			path: "/products",
			overrides: []app.CachePathOverride{
				{Path: "/products/*", TTL: 150},
				{Path: "/products", TTL: 100},
			},
			want: &app.CachePathOverride{Path: "/products", TTL: 100},
		},
		{
			name: "no match",
			path: "/orders",
			overrides: []app.CachePathOverride{
				{Path: "/products", TTL: 100},
				{Path: "/users", TTL: 200},
			},
			want: nil,
		},
		{
			name: "wildcard with nested path",
			path: "/products/123/reviews",
			overrides: []app.CachePathOverride{
				{Path: "/products/*", TTL: 150},
			},
			want: &app.CachePathOverride{Path: "/products/*", TTL: 150},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findPathOverride(tt.path, tt.overrides)
			if tt.want == nil {
				if got != nil {
					t.Errorf("findPathOverride() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("findPathOverride() = nil, want %v", tt.want)
				} else if got.Path != tt.want.Path || got.TTL != tt.want.TTL {
					t.Errorf("findPathOverride() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestGetPathWithoutPrefix(t *testing.T) {
	tests := []struct {
		name        string
		fullPath    string
		routePrefix string
		expected    string
	}{
		{
			name:        "simple path",
			fullPath:    "/api/products",
			routePrefix: "api",
			expected:    "/products",
		},
		{
			name:        "nested path",
			fullPath:    "/api/v1/users",
			routePrefix: "api",
			expected:    "/v1/users",
		},
		{
			name:        "path equals prefix",
			fullPath:    "/api",
			routePrefix: "api",
			expected:    "/",
		},
		{
			name:        "without leading slashes",
			fullPath:    "api/products",
			routePrefix: "api",
			expected:    "/products",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPathWithoutPrefix(tt.fullPath, tt.routePrefix)
			if got != tt.expected {
				t.Errorf("getPathWithoutPrefix() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sliceEqualInt(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

package cache

import (
	"net/http"
	"testing"
	"time"
)

func TestCacheEntry_IsFresh(t *testing.T) {
	tests := []struct {
		name     string
		entry    *CacheEntry
		expected bool
	}{
		{
			name: "fresh entry",
			entry: &CacheEntry{
				CachedAt: time.Now().Unix(),
				TTL:      300,
			},
			expected: true,
		},
		{
			name: "expired entry",
			entry: &CacheEntry{
				CachedAt: time.Now().Add(-600 * time.Second).Unix(),
				TTL:      300,
			},
			expected: false,
		},
		{
			name: "zero TTL",
			entry: &CacheEntry{
				CachedAt: time.Now().Unix(),
				TTL:      0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.IsFresh(); got != tt.expected {
				t.Errorf("CacheEntry.IsFresh() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCacheEntry_GetExpiryTime(t *testing.T) {
	now := time.Now()
	entry := &CacheEntry{
		CachedAt: now.Unix(),
		TTL:      300,
	}

	expectedExpiry := now.Add(300 * time.Second)
	gotExpiry := entry.GetExpiryTime()

	diff := gotExpiry.Sub(expectedExpiry)
	if diff > time.Second {
		t.Errorf("GetExpiryTime() = %v, want %v, diff = %v", gotExpiry, expectedExpiry, diff)
	}
}

func TestCacheEntry_Serialize(t *testing.T) {
	entry := &CacheEntry{
		StatusCode:   200,
		Headers:      map[string]string{"Content-Type": "application/json"},
		Body:         []byte(`{"test": true}`),
		CachedAt:     time.Now().Unix(),
		TTL:          300,
		ETag:         "abc123",
		LastModified: "Wed, 21 Oct 2015 07:28:00 GMT",
	}

	data, err := entry.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Serialize() returned empty data")
	}
}

func TestCacheEntry_Deserialize(t *testing.T) {
	original := &CacheEntry{
		StatusCode:   200,
		Headers:      map[string]string{"Content-Type": "application/json"},
		Body:         []byte(`{"test": true}`),
		CachedAt:     time.Now().Unix(),
		TTL:          300,
		ETag:         "abc123",
		LastModified: "Wed, 21 Oct 2015 07:28:00 GMT",
	}

	data, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	restored, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize() error = %v", err)
	}

	if restored.StatusCode != original.StatusCode {
		t.Errorf("StatusCode = %v, want %v", restored.StatusCode, original.StatusCode)
	}

	if restored.Headers["Content-Type"] != original.Headers["Content-Type"] {
		t.Errorf("Headers = %v, want %v", restored.Headers, original.Headers)
	}

	if string(restored.Body) != string(original.Body) {
		t.Errorf("Body = %v, want %v", string(restored.Body), string(original.Body))
	}

	if restored.TTL != original.TTL {
		t.Errorf("TTL = %v, want %v", restored.TTL, original.TTL)
	}
}

func TestNewCacheEntry(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("ETag", "abc123")
	headers.Add("Set-Cookie", "session=abc")
	headers.Add("Set-Cookie", "token=xyz")

	entry := NewCacheEntry(200, headers, []byte(`{"test": true}`), 300)

	if entry.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200", entry.StatusCode)
	}

	if entry.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type header = %v, want application/json", entry.Headers["Content-Type"])
	}

	if entry.ETag != "abc123" {
		t.Errorf("ETag = %v, want abc123", entry.ETag)
	}

	if string(entry.Body) != `{"test": true}` {
		t.Errorf("Body = %v, want {\"test\": true}", string(entry.Body))
	}

	if entry.TTL != 300 {
		t.Errorf("TTL = %v, want 300", entry.TTL)
	}

	expectedCachedAt := time.Now().Unix()
	diff := entry.CachedAt - expectedCachedAt
	if diff < -1 || diff > 1 {
		t.Errorf("CachedAt = %v, want ~%v", entry.CachedAt, expectedCachedAt)
	}

	expectedSetCookie := "session=abc, token=xyz"
	if entry.Headers["Set-Cookie"] != expectedSetCookie {
		t.Errorf("Set-Cookie header = %v, want %v", entry.Headers["Set-Cookie"], expectedSetCookie)
	}
}

func TestIsCacheable(t *testing.T) {
	tests := []struct {
		name       string
		config     *ResolvedCacheConfig
		statusCode int
		headers    http.Header
		bodySize   int
		want       bool
	}{
		{
			name: "cacheable response",
			config: &ResolvedCacheConfig{
				Enabled:     true,
				TTL:         300,
				StatusCodes: []int{200},
			},
			statusCode: 200,
			headers:    http.Header{},
			bodySize:   1024,
			want:       true,
		},
		{
			name: "disabled cache",
			config: &ResolvedCacheConfig{
				Enabled:     false,
				StatusCodes: []int{200},
			},
			statusCode: 200,
			headers:    http.Header{},
			bodySize:   1024,
			want:       false,
		},
		{
			name: "non-cacheable status code",
			config: &ResolvedCacheConfig{
				Enabled:     true,
				StatusCodes: []int{200},
			},
			statusCode: 404,
			headers:    http.Header{},
			bodySize:   1024,
			want:       false,
		},
		{
			name: "response exceeds max size",
			config: &ResolvedCacheConfig{
				Enabled:     true,
				StatusCodes: []int{200},
				MaxSize:     1024,
			},
			statusCode: 200,
			headers:    http.Header{},
			bodySize:   2048,
			want:       false,
		},
		{
			name: "no-store cache control",
			config: &ResolvedCacheConfig{
				Enabled:             true,
				StatusCodes:         []int{200},
				RespectCacheControl: true,
			},
			statusCode: 200,
			headers:    http.Header{"Cache-Control": []string{"no-store"}},
			bodySize:   1024,
			want:       false,
		},
		{
			name: "private cache control with respect enabled",
			config: &ResolvedCacheConfig{
				Enabled:             true,
				StatusCodes:         []int{200},
				RespectCacheControl: true,
			},
			statusCode: 200,
			headers:    http.Header{"Cache-Control": []string{"private"}},
			bodySize:   1024,
			want:       false,
		},
		{
			name: "private cache control with respect disabled",
			config: &ResolvedCacheConfig{
				Enabled:             true,
				StatusCodes:         []int{200},
				RespectCacheControl: false,
			},
			statusCode: 200,
			headers:    http.Header{"Cache-Control": []string{"private"}},
			bodySize:   1024,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCacheable(tt.config, tt.statusCode, tt.headers, tt.bodySize)
			if got != tt.want {
				t.Errorf("IsCacheable() = %v, want %v", got, tt.want)
			}
		})
	}
}

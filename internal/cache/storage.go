package cache

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CacheEntry represents a cached HTTP response
type CacheEntry struct {
	StatusCode   int               `json:"status_code"`
	Headers      map[string]string `json:"headers"`
	Body         []byte            `json:"body"`
	CachedAt     int64             `json:"cached_at"`
	TTL          int               `json:"ttl"`
	ETag         string            `json:"etag,omitempty"`
	LastModified string            `json:"last_modified,omitempty"`
}

// IsFresh checks if the cache entry is still fresh (not expired)
func (e *CacheEntry) IsFresh() bool {
	if e.TTL <= 0 {
		return false
	}

	cachedTime := time.Unix(e.CachedAt, 0)
	expiryTime := cachedTime.Add(time.Duration(e.TTL) * time.Second)

	return time.Now().Before(expiryTime)
}

// GetExpiryTime returns the expiry time of the cache entry
func (e *CacheEntry) GetExpiryTime() time.Time {
	cachedTime := time.Unix(e.CachedAt, 0)
	return cachedTime.Add(time.Duration(e.TTL) * time.Second)
}

// Serialize converts the cache entry to JSON
func (e *CacheEntry) Serialize() ([]byte, error) {
	return json.Marshal(e)
}

// Deserialize converts JSON to a cache entry
func Deserialize(data []byte) (*CacheEntry, error) {
	var entry CacheEntry
	err := json.Unmarshal(data, &entry)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// NewCacheEntry creates a new cache entry from HTTP response
func NewCacheEntry(statusCode int, headers http.Header, body []byte, ttl int) *CacheEntry {
	// Convert http.Header to map[string]string for JSON serialization
	// Concatenate multi-value headers with a separator
	headerMap := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			if len(values) == 1 {
				headerMap[key] = values[0]
			} else {
				// Multiple values - concatenate with separator
				headerMap[key] = strings.Join(values, ", ")
			}
		}
	}

	return &CacheEntry{
		StatusCode:   statusCode,
		Headers:      headerMap,
		Body:         body,
		CachedAt:     time.Now().Unix(),
		TTL:          ttl,
		ETag:         headers.Get("ETag"),
		LastModified: headers.Get("Last-Modified"),
	}
}

// CacheSizeExceededError is returned when response size exceeds maximum cache size
type CacheSizeExceededError struct {
	Size    int
	MaxSize int
}

func (e *CacheSizeExceededError) Error() string {
	return fmt.Sprintf("cache size %d bytes exceeds maximum %d bytes", e.Size, e.MaxSize)
}

// IsCacheable checks if the HTTP response is cacheable
func IsCacheable(config *ResolvedCacheConfig, statusCode int, headers http.Header, bodySize int) bool {
	// Check if caching is enabled
	if !config.Enabled {
		return false
	}

	// Check size limit
	if config.MaxSize > 0 && bodySize > config.MaxSize {
		return false
	}

	// Check status code
	statusCodeCacheable := false
	for _, code := range config.StatusCodes {
		if statusCode == code {
			statusCodeCacheable = true
			break
		}
	}
	if !statusCodeCacheable {
		return false
	}

	// Respect Cache-Control if configured
	if config.RespectCacheControl {
		cacheControl := headers.Get("Cache-Control")
		if strings.Contains(cacheControl, "no-store") ||
			strings.Contains(cacheControl, "private") ||
			strings.Contains(cacheControl, "no-cache") {
			return false
		}
	}

	return true
}

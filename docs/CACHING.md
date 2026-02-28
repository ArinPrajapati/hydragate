# Caching in HydraGate

## Overview

HydraGate implements a sophisticated three-layer caching system using Redis to significantly improve API performance and reduce backend load. The caching layer sits between authentication and the reverse proxy, ensuring both security and performance.

## Architecture

```
Client Request
    ↓
[Auth Middleware] ← JWT/API Key validation
    ↓
[Cache Middleware] ← Redis caching (3-layer config)
    ↓
[Rate Limiter]
    ↓
[Proxy] ← Backend service
```

## Three-Layer Configuration System

HydraGate uses a **hierarchical configuration model** with three levels of precedence:

```
Global Defaults (Level 1)
    ↓
Route-Level Override (Level 2)
    ↓
Path-Level Override (Level 3) ← Highest precedence
```

### Level 1: Global Default Configuration

Applied to all routes unless overridden. Configured at the gateway level.

```json
{
  "cache": {
    "enabled": true,
    "default_ttl": 300,
    "methods": ["GET"],
    "status_codes": [200, 304],
    "key": {
      "include_query": true,
      "include_headers": ["Accept", "Accept-Language"],
      "respect_cache_control": true,
      "etag_validation": false
    },
    "max_size": 1048576
  }
}
```

**Purpose**: Provides sensible defaults for the entire gateway, reducing configuration overhead.

---

### Level 2: Route-Level Configuration

Overrides global defaults for specific route prefixes.

```json
{
  "routes": [
    {
      "route": "/api/products",
      "target": "http://products-service:8000",
      "cache": {
        "enabled": true,
        "ttl": 600,
        "methods": ["GET", "HEAD"],
        "status_codes": [200],
        "key": {
          "include_query": false,
          "include_headers": ["Accept"]
        }
      }
    }
  ]
}
```

**Purpose**: Allows different caching strategies per service or route group (e.g., longer TTL for rarely-changing data).

---

### Level 3: Path-Level Configuration

Highest precedence - overrides both global and route-level settings for specific paths.

```json
{
  "route": "/api/products",
  "target": "http://products-service:8000",
  "cache": {
    "enabled": true,
    "ttl": 600
  },
  "cache_paths": [
    {
      "path": "/products/list",
      "enabled": true,
      "ttl": 900,
      "methods": ["GET"]
    },
    {
      "path": "/products/*",
      "enabled": true,
      "ttl": 1200,
      "key": {
        "include_query": true,
        "include_headers": ["Authorization"]
      }
    }
  ]
}
```

**Supports**:
- **Exact matches**: `/products/list` matches only that path
- **Wildcard matches**: `/products/*` matches `/products/123`, `/products/123/reviews`, etc.
- **Exact precedence**: Exact matches take priority over wildcards

**Purpose**: Fine-grained control for specific endpoints (e.g., disable caching for real-time data, shorter TTL for frequently-changing content).

---

## Configuration Resolution Logic

The cache configuration is resolved at runtime by merging the three levels:

```
1. Start with global defaults
2. Apply route-level overrides (if any)
3. Apply path-level overrides (if any match)
4. Return final resolved configuration
```

### Example: Configuration Flow

**Global Configuration:**
```json
{
  "cache": {
    "enabled": true,
    "default_ttl": 300,
    "methods": ["GET"],
    "key": { "include_query": false }
  }
}
```

**Route Configuration for `/api/users`:**
```json
{
  "cache": {
    "ttl": 600,
    "methods": ["GET", "HEAD"]
  }
}
```

**Path-Level Override for `/api/users/profile`:**
```json
{
  "cache_paths": [
    {
      "path": "/users/profile",
      "ttl": 1200,
      "key": { "include_query": true }
    }
  ]
}
```

**Resulting Configuration:**
| Path | Enabled | TTL | Methods | Include Query |
|------|---------|-----|---------|---------------|
| `/api/users/list` | ✅ true | 600s | GET, HEAD | ❌ false |
| `/api/users/profile` | ✅ true | 1200s | GET, HEAD | ✅ true |
| `/api/users/123` | ✅ true | 600s | GET, HEAD | ❌ false |

---

## Cache Key Generation

Cache keys are human-readable and include multiple components to ensure cache correctness.

### Key Format

```
gateway:cache:{method}:{route_prefix}:{path_segments}:{query?}:{headers?}:{user_identity?}
```

### Components

1. **Method**: HTTP method (GET, POST, etc.)
2. **Route Prefix**: The matched route prefix (e.g., `api`, `products`)
3. **Path Segments**: Remaining path components
4. **Query Parameters**: Normalized and sorted (if configured)
5. **Headers**: Configured headers, normalized (if configured)
6. **User Identity**: User ID/API key for protected routes

### Examples

| Request | Cache Key |
|---------|-----------|
| `GET /api/products` | `gateway:cache:GET:api:products` |
| `GET /api/products?page=2&limit=10` | `gateway:cache:GET:api:products:limit=10&page=2` |
| `GET /api/products/123` (protected) | `gateway:cache:GET:api:products:123:user456` |
| `GET /api/products` with Accept header | `gateway:cache:GET:api:products:accept:application/json` |

### Key Features

- **Query Parameter Sorting**: `page=2&limit=10` and `limit=10&page=2` generate identical keys
- **Header Normalization**: Headers are lowercased and whitespace-trimmed
- **User Scoping**: Protected routes include user identity to prevent data leakage

---

## Security: User Identity in Cache Keys

For **protected routes** (routes requiring authentication), cache keys include user identity to prevent cache poisoning and data leakage.

### Identity Sources

1. **JWT Claims**: Extracted from forwarded headers after JWT validation
   - `X-User-Id`
   - `X-User-Sub`
   - `X-User-Email`

2. **API Keys**: Extracted from API key authentication header
   - `X-API-Key-Name` → prefixed as `apikey:{name}`

3. **Anonymous**: If no identity found, uses `anonymous` label

### Implementation

```go
// Cache key for protected route
GET /api/products/123
Header: X-User-Id: user456
Key: gateway:cache:GET:api:products:123:user456

// Cache key for another user
GET /api/products/123
Header: X-User-Id: user789
Key: gateway:cache:GET:api:products:123:user789
```

**Benefits**:
- User A's cached data is never served to User B
- Prevents unauthorized data access
- Prevents cache poisoning attacks

---

## Cache Invalidation

HydraGate provides multiple methods to invalidate cached data.

### 1. Delete Specific Key

Remove a single cached entry by its cache key.

```go
cacheClient.Delete(ctx, "GET:api:products:123:user456")
```

### 2. Delete by Pattern

Remove all entries matching a wildcard pattern.

```go
// Delete all product-related cache entries
cacheClient.DeletePattern(ctx, "GET:api:products:*")

// Delete cache for specific user
cacheClient.DeletePattern(ctx, "*:user456")
```

**Pattern Syntax**: Uses Redis SCAN with pattern matching
- `*` matches any characters
- Pattern is applied after the `gateway:cache:` prefix

### 3. Flush by Route Prefix

Clear all cache entries for a specific route.

```go
// Clear all /api/users cache
cacheClient.FlushPrefix(ctx, "api:users")

// This deletes:
// - GET:api:users:list
// - GET:api:users/123
// - GET:api:users/123/orders
// etc.
```

### 4. Flush All Cache

Remove all cache entries managed by the gateway.

```go
cacheClient.FlushAll(ctx)
```

**Use Cases**:
- After deployment of new backend version
- After bulk data updates
- Cache corruption recovery

---

## Cache Storage Structure

### Redis Key Format

```
gateway:cache:{cache_key}
```

### Entry Structure

Each cache entry is stored as JSON with the following structure:

```json
{
  "status_code": 200,
  "headers": {
    "Content-Type": "application/json",
    "Set-Cookie": "session=abc, token=xyz"
  },
  "body": "{\"id\": 123, \"name\": \"Product\"}",
  "cached_at": 1640000000,
  "ttl": 300,
  "etag": "\"abc123\"",
  "last_modified": "Wed, 21 Oct 2015 07:28:00 GMT"
}
```

### Fields

- `status_code`: HTTP status code of cached response
- `headers`: Response headers (multi-value headers concatenated with `, `)
- `body`: Response body bytes
- `cached_at`: Unix timestamp of when entry was cached
- `ttl`: Time-to-live in seconds
- `etag`: ETag header value (if present)
- `last_modified`: Last-Modified header value (if present)

---

## Middleware Integration

### Middleware Order

```go
middleware.Chain(
    http.HandlerFunc(proxy.Forward(reg)),
    middleware.Logger,
    rateLimiter,
    jwtAuth,
    apiKeyAuth,
    cacheMiddleware,  // ← Cache runs AFTER authentication
)
```

**Why this order?**
1. **Proxy/Logger**: First to handle requests
2. **Rate Limiter**: Apply limits early
3. **JWT/API Key Auth**: Authenticate requests
4. **Cache**: Now user identity is known → can safely cache per-user data
5. **Backend Proxy**: Only reached on cache miss

### Cache Flow

```
Request arrives
    ↓
Check Redis health
    ↓ (down)
Skip cache, proxy to backend
    ↓ (up)
Parse path, get route config
    ↓ (no route)
Skip cache, proxy to backend
    ↓
Resolve cache config (3-layer)
    ↓ (not cacheable)
Skip cache, proxy to backend
    ↓
Extract user identity (if protected)
    ↓
Generate cache key
    ↓
Try to get from Redis
    ↓ (cache hit)
Serve cached response ✓
    ↓ (cache miss)
Proxy to backend
    ↓
Check if response is cacheable
    ↓ (not cacheable)
Return response (don't cache)
    ↓ (cacheable)
Store in Redis
Return response ✓
```

---

## Configuration Options

### Complete Configuration Schema

```json
{
  "cache": {
    "enabled": true,
    "default_ttl": 300,
    "methods": ["GET"],
    "status_codes": [200, 304],
    "key": {
      "include_query": true,
      "include_headers": ["Accept", "Accept-Language"],
      "respect_cache_control": true,
      "etag_validation": false
    },
    "max_size": 1048576
  },
  "routes": [
    {
      "route": "/api/products",
      "target": "http://products-service:8000",
      "protected": true,
      "cache": {
        "enabled": true,
        "ttl": 600,
        "methods": ["GET", "HEAD"],
        "status_codes": [200],
        "key": {
          "include_query": false,
          "include_headers": ["Accept"],
          "respect_cache_control": true,
          "etag_validation": false
        }
      },
      "cache_paths": [
        {
          "path": "/products/list",
          "enabled": true,
          "ttl": 900,
          "methods": ["GET"],
          "status_codes": [200],
          "key": {
            "include_query": true,
            "include_headers": []
          }
        },
        {
          "path": "/products/*",
          "enabled": true,
          "ttl": 1200,
          "methods": ["GET"]
        }
      ]
    }
  ]
}
```

### Option Explanations

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `enabled` | bool | Enable/disable caching | true |
| `ttl` | int | Time-to-live in seconds | 300 |
| `methods` | array | HTTP methods to cache | ["GET"] |
| `status_codes` | array | Response status codes to cache | [200, 304] |
| `include_query` | bool | Include query params in cache key | false |
| `include_headers` | array | Headers to include in cache key | [] |
| `respect_cache_control` | bool | Honor backend Cache-Control headers | true |
| `etag_validation` | bool | Enable ETag validation | false |
| `max_size` | int | Maximum response size to cache (bytes) | 1048576 |

---

## Cache Control Header Support

When `respect_cache_control` is enabled, the gateway respects backend `Cache-Control` headers:

| Cache-Control Value | Behavior |
|--------------------|----------|
| `no-store` | Response not cached |
| `private` | Response not cached |
| `no-cache` | Response not cached |
| `public` | Response cached normally |
| `max-age=60` | Uses backend's max-age if smaller than configured TTL |

**Example:**
```
Backend Response:
Cache-Control: no-store

Gateway Behavior:
Response not cached, served directly from backend
```

---

## Multi-Value Header Handling

HydraGate properly handles headers with multiple values (e.g., `Set-Cookie`).

### Storage

Multiple values are concatenated with `, ` separator:
```json
{
  "headers": {
    "Set-Cookie": "session=abc, token=xyz"
  }
}
```

### Retrieval

Headers are split back when serving from cache:
```go
// Split multi-value headers
values := strings.Split("session=abc, token=xyz", ", ")
for _, v := range values {
    w.Header().Add("Set-Cookie", v)
}
```

**Result**: All headers are correctly reconstructed when serving cached responses.

---

## Redis Integration

### Connection Configuration

```go
rdb := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
    // Additional options:
    // Password: "",
    // DB: 0,
    // PoolSize: 10,
})
```

### Health Check

The cache middleware performs health checks on every request:

```go
if !cacheClient.IsHealthy(ctx) {
    logRedisWarning("cache Redis is down, skipping cache")
    next.ServeHTTP(w, r)  // Graceful degradation
    return
}
```

**Behavior on Redis Failure:**
- Throttled warning logs (every 10 seconds)
- Requests bypass cache and go directly to backend
- No error returned to clients
- Gateway continues operating normally

---

## Performance Considerations

### Benefits

1. **Reduced Backend Load**: Cached responses don't hit backend services
2. **Lower Latency**: Redis provides millisecond-level response times
3. **Scalability**: Gateway can handle more concurrent requests
4. **Cost Savings**: Fewer backend resources needed

### Best Practices

1. **TTL Selection**:
   - Static content (docs, assets): 1-24 hours
   - Product lists: 5-15 minutes
   - User profiles: 1-5 minutes
   - Real-time data: Do not cache

2. **Query Parameter Inclusion**:
   - Include only if backend responses vary by query params
   - Excluding reduces cache key complexity and improves hit rates

3. **Header Inclusion**:
   - Include only headers that affect response content (e.g., `Accept`)
   - Don't include headers like `User-Agent` that don't affect responses

4. **Max Size Limits**:
   - Set reasonable limits to prevent memory bloat
   - Large responses (1MB+) should not be cached

5. **Cache Invalidation**:
   - Use pattern-based invalidation after bulk updates
   - Implement cache invalidation hooks in backend services
   - Consider TTL-based expiration for simplicity

---

## Monitoring and Logging

### Cache Hit Logging

```json
{
  "level": "info",
  "msg": "cache hit",
  "key": "gateway:cache:GET:api:products:user456",
  "path": "/api/products",
  "method": "GET",
  "user": "user456"
}
```

### Cache Miss Logging

```json
{
  "level": "info",
  "msg": "cache stored",
  "key": "gateway:cache:GET:api:products:user456",
  "path": "/api/products",
  "ttl": 300,
  "user": "user456"
}
```

### Cache Skip Logging

```json
{
  "level": "debug",
  "msg": "response not cacheable",
  "key": "gateway:cache:GET:api:products:user456",
  "status": 404
}
```

### Redis Health Logging

```json
{
  "level": "warn",
  "msg": "cache Redis is down, skipping cache"
}
```
*(Logged only once every 10 seconds to prevent log spam)*

---

## Error Handling

### Graceful Degradation

The cache system is designed to fail gracefully:

1. **Redis Connection Failed**: Requests bypass cache, served from backend
2. **Serialization Error**: Response not cached, served to client
3. **Deserialization Error**: Cache entry ignored, served from backend
4. **Size Limit Exceeded**: Response not cached, served to client

**No errors are returned to clients** - the gateway continues operating normally.

---

## Testing

HydraGate includes comprehensive test coverage for the cache system:

### Test Files

- `internal/cache/storage_test.go` - CacheEntry operations, serialization, cacheability
- `internal/cache/key_test.go` - Key generation, normalization, user identity
- `internal/cache/config_test.go` - Configuration resolution, path overrides

### Test Coverage

```
✓ CacheEntry IsFresh()
✓ CacheEntry GetExpiryTime()
✓ CacheEntry Serialize/Deserialize()
✓ NewCacheEntry()
✓ IsCacheable()
✓ GenerateCacheKey()
✓ normalizePath()
✓ removeRoutePrefix()
✓ normalizeQuery()
✓ normalizeHeaders()
✓ extractUserIdentity()
✓ ResolveCacheConfig()
✓ findPathOverride()
✓ getPathWithoutPrefix()
```

### Running Tests

```bash
# Run all cache tests
go test ./internal/cache/... -v

# Run specific test
go test ./internal/cache/... -v -run TestGenerateCacheKey
```

---

## Example Scenarios

### Scenario 1: E-commerce Product Listing

**Requirement**: Cache product listings for 10 minutes, but exclude personalized recommendations.

**Configuration**:
```json
{
  "cache_paths": [
    {
      "path": "/products/list",
      "ttl": 600,
      "key": {
        "include_query": true,  // Include filters (category, price)
        "include_headers": ["Accept"]
      }
    },
    {
      "path": "/products/recommendations",
      "enabled": false  // Never cache personalized content
    }
  ]
}
```

---

### Scenario 2: User Profile API

**Requirement**: Cache user profiles with user-specific cache keys, invalidate on profile updates.

**Configuration**:
```json
{
  "route": "/api/users",
  "protected": true,
  "cache": {
    "ttl": 300,
    "methods": ["GET"]
  }
}
```

**Cache Keys**:
- User 456: `gateway:cache:GET:api:users:profile:user456`
- User 789: `gateway:cache:GET:api:users:profile:user789`

**Invalidation** (after user updates profile):
```go
cacheClient.Delete(ctx, "GET:api:users:profile:user456")
```

---

### Scenario 3: API Versioning

**Requirement**: Different TTL for different API versions.

**Configuration**:
```json
{
  "routes": [
    {
      "route": "/api/v1/products",
      "cache": {
        "ttl": 1800  // 30 min for stable v1
      }
    },
    {
      "route": "/api/v2/products",
      "cache": {
        "ttl": 60  // 1 min for frequently-changing v2
      }
    }
  ]
}
```

---

## Migration Guide

### Disabling Caching

To disable caching for a specific route:

**Method 1**: Route-level
```json
{
  "cache": {
    "enabled": false
  }
}
```

**Method 2**: Path-level
```json
{
  "cache_paths": [
    {
      "path": "/real-time/updates",
      "enabled": false
    }
  ]
}
```

### Increasing TTL

Increase cache duration to reduce backend load:

```json
{
  "cache": {
    "default_ttl": 600  // Increase from 300 to 600 seconds
  }
}
```

**Note**: Existing cached entries will expire at their original TTL. New entries use the increased TTL.

---

## Troubleshooting

### Cache Not Working

**Symptoms**: All requests hit backend, no cache hits logged.

**Checks**:
1. Verify `cache.enabled` is true
2. Verify Redis is running: `redis-cli ping`
3. Check gateway logs for Redis health warnings
4. Ensure request method is in `cache.methods` list
5. Verify route configuration is loaded correctly

### Wrong Data Returned

**Symptoms**: User A receives User B's data.

**Checks**:
1. Verify route is marked as `protected: true`
2. Ensure cache middleware runs AFTER auth middleware
3. Check that user identity headers are being set by auth middleware
4. Verify cache keys include user identity

### Cache Invalidation Not Working

**Symptoms**: Old data still served after updates.

**Checks**:
1. Verify invalidation pattern is correct
2. Check Redis keys: `redis-cli keys "gateway:cache:*"`
3. Ensure Redis DEL command is successful
4. Consider using shorter TTL as fallback

### High Memory Usage

**Symptoms**: Redis memory usage growing unbounded.

**Solutions**:
1. Reduce `default_ttl` and route-specific TTLs
2. Set `max_size` to limit cached response sizes
3. Implement periodic `FlushPattern` for old data
4. Enable Redis maxmemory policy: `maxmemory-policy allkeys-lru`

---

## Future Enhancements

Potential improvements for future versions:

1. **ETag Validation**: Implement conditional requests (If-None-Match)
2. **Cache Warmer**: Background job to pre-populate cache
3. **Distributed Cache**: Support for multiple Redis instances
4. **Compression**: Compress large cache entries
5. **Metrics**: Export Prometheus metrics for cache hits/misses
6. **Cache Analytics**: Dashboard showing cache performance
7. **Bloom Filters**: Fast cache key existence checks
8. **Stale-While-Revalidate**: Serve stale cache while refreshing
9. **Cache Tags**: Tag-based invalidation (e.g., tag with product IDs)
10. **Edge Cache**: Support for CDN edge caching integration

---

## Conclusion

HydraGate's three-layer caching system provides:

- ✅ **Flexibility**: Fine-grained control at multiple levels
- ✅ **Security**: User-scoped cache keys prevent data leakage
- ✅ **Performance**: Reduces backend load and latency
- ✅ **Reliability**: Graceful degradation on Redis failures
- ✅ **Maintainability**: Clear configuration hierarchy
- ✅ **Observability**: Comprehensive logging and monitoring

This caching implementation is production-ready and suitable for high-traffic API gateways.

# HydraGate

A production-grade API Gateway built in Go.

HydraGate sits between clients and backend services, acting as the single entry point for all traffic.

```
Client → HydraGate → Backend Services
```

**Project Progress:** Phase 1 ✅ | Phase 2 ✅ | Phase 3.1 ✅ | Phase 3.2 🔜

---

## Current Status — Phase 1: Core Gateway Foundation ✅

| Component                 | Status  |
| ------------------------- | ------- |
| HTTP Server               | ✅ Done |
| Middleware system (Chain) | ✅ Done |
| Request Logger            | ✅ Done |
| Reverse Proxy             | ✅ Done |
| Route Registry            | ✅ Done |

---

## Current Status — Phase 2: Production Features ✅

| Component                 | Status  |
| ------------------------- | ------- |
| JWT authentication        | ✅ Done |
| API key authentication    | ✅ Done |
| Rate limiting (Redis)      | ✅ Done |
| Structured logging        | ✅ Done |
| Request transform         | ✅ Done |
| Config reload              | ✅ Done |

---

## Current Status — Phase 3.1: Caching (Redis) ✅

| Component                 | Status  |
| ------------------------- | ------- |
| Redis caching middleware   | ✅ Done |
| Cache key generation      | ✅ Done |
| Cache configuration       | ✅ Done |
| Per-route cache control   | ✅ Done |

---

## Project Structure

```
hydragate/
├── cmd/
│   └── server/
│       └── main.go          # Entry point, middleware chain, server setup
├── internal/
│   ├── app/
│   │   └── app.go           # Application types (RouteConfig, CacheConfig)
│   ├── auth/
│   │   └── auth.go          # JWT claim extraction utilities
│   ├── cache/
│   │   ├── config.go        # Cache configuration types
│   │   ├── key.go           # Cache key generation
│   │   ├── middleware.go    # Redis cache middleware
│   │   └── redis.go         # Redis client wrapper
│   ├── config/
│   │   ├── loader.go        # Config file loader (JSON)
│   │   ├── state.go         # Thread-safe config state
│   │   ├── watcher.go       # File watcher for hot-reload
│   │   └── reload.go        # Reload validation and swap
│   ├── middleware/
│   │   ├── Chain.go         # Middleware chaining
│   │   ├── Logger.go        # Request logger (method, status, latency, UUID)
│   │   ├── JWTAuth.go       # JWT authentication
│   │   ├── APIKeyAuth.go    # API key authentication
│   │   └── RateLimiter.go   # Rate limiting with Redis
│   ├── proxy/
│   │   ├── registry.go      # Route registry (prefix → target URL)
│   │   └── forward.go       # Reverse proxy logic
│   └── urlpath/
│       └── urlpath.go       # URL path utilities
├── config.json              # Configuration file
├── docker-compose.yml       # Redis container setup
├── product.md               # Full feature roadmap
└── progess.txt              # Development progress log
```

---

## How It Works

1. **Server** starts on `:8080` and registers routes.
2. Every request passes through the **middleware chain** (Logger → RateLimiter → JWTAuth → APIKeyAuth → Cache → Proxy).
3. The **proxy** resolves the request's path prefix against the route registry and forwards it to the correct backend.
4. **Logger** captures method, status code, latency, and a unique request ID (UUID) for every request.
5. **Cache** stores GET responses in Redis for configured routes to reduce backend load.

### Example routing config (in `config.json`)

```json
{
  "routes": [
    {
      "prefix": "api",
      "target": "http://localhost:9000",
      "protected": true,
      "cache": {
        "enabled": true,
        "ttl": 300
      }
    }
  ]
}
```

### Health check

```
GET /health → "Alive"
```

### Config Reload

HydraGate supports hot-reloading of the configuration without server restart:

**Automatic reload:** Changes to `config.json` are automatically detected and applied.

**Manual reload:** Trigger via HTTP endpoint:
```bash
curl -X POST http://localhost:8080/reload
```

**Reload behavior:**
- Thread-safe atomic config swap (no request downtime)
- Validates new config before applying
- Updates all middleware (JWT auth, API keys, rate limiting, routes, cache)
- Logs reload events

### Caching

HydraGate supports Redis-backed response caching:

**Per-route cache control:** Enable/disable caching per route in config
**TTL support:** Configure cache expiration time per route
**Smart key generation:** Cache keys include method, path, and query parameters
**Cache middleware:** Automatically checks cache before proxying requests

**Example cached route:**
```json
{
  "prefix": "users",
  "target": "http://users-service:8001",
  "cache": {
    "enabled": true,
    "ttl": 300
  }
}
```

## Configuration

HydraGate uses `config.json` for all configuration:

```json
{
  "jwt_secret": "your-secret-key",
  "api_keys": ["key1", "key2"],
  "rate_limit": {
    "requests_per_minute": 100,
    "window_minutes": 1
  },
  "forward_claims": ["user_id", "role"],
  "routes": [
    {
      "prefix": "api",
      "target": "http://localhost:9000",
      "protected": true,
      "transform": {
        "add_headers": {"X-Gateway": "HydraGate"},
        "remove_headers": ["X-Internal"]
      },
      "cache": {
        "enabled": true,
        "ttl": 300
      }
    }
  ]
}
```

**Configuration options:**
- `jwt_secret`: Secret key for JWT token validation
- `api_keys`: List of valid API keys
- `rate_limit`: Rate limiting configuration
- `forward_claims`: JWT claims to forward to backend
- `routes`: List of route definitions
  - `prefix`: URL path prefix
  - `target`: Backend service URL
  - `protected`: Require authentication (true/false)
  - `transform`: Request/response transformations
  - `cache`: Cache configuration

---

## Run

**Prerequisites:**
- Go 1.21+
- Redis (running on localhost:6379)

**Start Redis:**
```bash
docker-compose up -d
```

**Run HydraGate:**
```bash
go run ./cmd/server
```

Server will start on `http://localhost:8080`

---

## Roadmap

### 🟢 Phase 1 — Core Gateway Foundation ✅

- HTTP server ✅
- Middleware system ✅
- Reverse proxy ✅
- Route registry ✅
- Request logging ✅

### 🟢 Phase 2 — Production Features ✅

- JWT authentication ✅
- API key system ✅
- Rate limiting (Redis) ✅
- Structured logging ✅
- Request/response transform ✅
- Config hot-reload ✅

### 🟢 Phase 3.1 — Caching (Redis) ✅

- Redis caching middleware ✅
- Per-route cache control ✅
- Cache key generation ✅
- Docker setup (Redis) ✅

### 🔴 Phase 3.2 — Advanced Features 🔜

- Plugin system
- Load balancing
- Circuit breaker
- Prometheus metrics
- API key management (admin REST API)
- Optional dashboard

## 📋 Future Development Roadmap

For detailed future development plans, see [FUTURE_ROADMAP.md](docs/FUTURE_ROADMAP.md).

### Short-Term Goals (1-3 months)
1. **Complete Plugin System** - Framework designed, needs full implementation with .so plugin support
2. **Load Balancing** - Multiple strategies (round-robin, least-connections) with health checks
3. **Circuit Breaker** - Prevent cascading failures with automatic recovery

### Medium-Term Goals (3-6 months)
1. **Prometheus Metrics** - Comprehensive observability with Grafana dashboards
2. **API Key Management API** - REST API for dynamic key management
3. **Request Retry** - Automatic retry with exponential backoff

### Long-Term Goals (6+ months)
1. **Web Dashboard** - UI for monitoring and management
2. **Service Discovery** - Integration with Consul and Kubernetes
3. **WebSocket Support** - Proper WebSocket proxying and metrics
4. **Advanced Security** - IP filtering, request signatures, CORS, security headers
5. **Performance Optimization** - Connection pooling, HTTP/2, compression
6. **Developer Experience** - Better tools, documentation, and examples

See [IMPLEMENTATION_CHECKLIST.md](docs/IMPLEMENTATION_CHECKLIST.md) for a complete implementation checklist.

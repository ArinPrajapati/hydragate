# HydraGate

A production-grade API Gateway built in Go.

HydraGate sits between clients and backend services, acting as the single entry point for all traffic.

```
Client → HydraGate → Backend Services
```

**Project Progress:** Phase 1 ✅ | Phase 2 ✅ | Phase 3.1 ✅ | Phase 3.2 (Plugin System) ✅ | Phase 3.3 🔜

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

| Component               | Status  |
| ----------------------- | ------- |
| JWT authentication      | ✅ Done |
| API key authentication  | ✅ Done |
| Rate limiting (Redis)   | ✅ Done |
| Structured logging      | ✅ Done |
| Request transform       | ✅ Done |
| Config hot-reload       | ✅ Done |

---

## Current Status — Phase 3.1: Caching (Redis) ✅

| Component               | Status  |
| ----------------------- | ------- |
| Redis caching middleware | ✅ Done |
| Cache key generation    | ✅ Done |
| Cache configuration     | ✅ Done |
| Per-route cache control | ✅ Done |

---

## Current Status — Phase 3.2: Plugin System ✅

| Component                          | Status  |
| ---------------------------------- | ------- |
| Plugin interface + 4-phase lifecycle | ✅ Done |
| Plugin registry (built-in + `.so`) | ✅ Done |
| Plugin executor (timeout, priority) | ✅ Done |
| Full HTTP middleware integration    | ✅ Done |
| ResponseCapture (buffered response) | ✅ Done |
| BasePlugin (no-op embed)           | ✅ Done |
| LoggerPlugin (built-in)            | ✅ Done |
| RateLimiterPlugin (built-in)       | ✅ Done |
| JWTAuthPlugin (built-in)           | ✅ Done |
| APIKeyAuthPlugin (built-in)        | ✅ Done |
| Hot-reload of plugin config        | ✅ Done |

---

## Project Structure

```
hydragate/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: registers plugins, wires handler chain
├── internal/
│   ├── app/
│   │   └── app.go               # All config types (GatewayConfig, RouteConfig, PluginsConfig...)
│   ├── auth/
│   │   └── auth.go              # JWT token validation (used by JWTAuthPlugin)
│   ├── cache/
│   │   ├── config.go            # Cache config resolution (global → route → path override)
│   │   ├── key.go               # Cache key generation
│   │   ├── middleware.go        # Redis cache middleware (standalone, wraps proxy)
│   │   ├── redis.go             # Redis client wrapper + health check
│   │   └── storage.go           # CacheEntry type + freshness check
│   ├── config/
│   │   ├── loader.go            # Config file loader (JSON → GatewayConfig)
│   │   ├── prase.go             # JSON parsing
│   │   ├── state.go             # Thread-safe atomic State (config + registry + executor)
│   │   ├── watcher.go           # fsnotify file watcher → debounced hot-reload
│   │   └── reload.go            # Reload: validate → rebuild registry + executor → swap
│   ├── middleware/
│   │   ├── Chain.go             # Simple middleware chaining helper
│   │   ├── Logger.go            # (legacy standalone logger, superseded by LoggerPlugin)
│   │   ├── JWTAuth.go           # (legacy standalone JWT, superseded by JWTAuthPlugin)
│   │   ├── APIKeyAuth.go        # (legacy standalone API key, superseded by APIKeyAuthPlugin)
│   │   ├── RateLimiter.go       # (legacy standalone rate limiter, superseded by RateLimiterPlugin)
│   │   └── rate_limit.lua       # Redis token-bucket Lua script
│   ├── plugin/
│   │   ├── types.go             # Plugin interface, PluginContext, PluginPhase constants
│   │   ├── factory.go           # PluginFactory type definition
│   │   ├── registry.go          # Factory registration + external .so loading
│   │   ├── executor.go          # Phase execution engine: timeout, priority sort, on_error
│   │   ├── middleware.go        # 4-phase HTTP middleware (PreRoute→PreUpstream→PostUpstream→PreResponse→Flush)
│   │   └── response.go          # ResponseCapture: buffers backend response for post-upstream phases
│   ├── plugins/                 # Built-in plugin implementations
│   │   ├── base.go              # BasePlugin — embed for no-op defaults
│   │   ├── logger.go            # LoggerPlugin: OnPreRoute + OnPreResponse
│   │   ├── rate_limiter.go      # RateLimiterPlugin: OnPreRoute, Redis token bucket
│   │   ├── jwt_auth.go          # JWTAuthPlugin: OnPreUpstream, claim forwarding
│   │   ├── api_key_auth.go      # APIKeyAuthPlugin: OnPreUpstream, X-API-Key header
│   │   └── rate_limit.lua       # Embedded Lua script (copy of middleware/rate_limit.lua)
│   ├── proxy/
│   │   ├── registry.go          # Route registry (prefix → target + transform + cache config)
│   │   └── forward.go           # Reverse proxy: path rewrite, header transform, upstream call
│   └── urlpath/
│       └── urlpath.go           # URL path parser: prefix / path / query
├── config.json                  # Gateway configuration (routes, auth, cache, plugins)
├── docker-compose.yml           # Redis container
├── product.md                   # Full feature roadmap
└── progess.txt                  # Development progress log
```

---

## How It Works

### Request Lifecycle

Every request passes through a structured 4-phase plugin pipeline before reaching the backend:

```
Client Request
      │
      ▼
┌─────────────────────────────────────────┐
│  X-Request-ID injected (UUID)           │
└──────────────────┬──────────────────────┘
                   │
                   ▼
┌──────────────────────────────┐
│  PhasePreRoute               │  Global plugins only (route not matched yet)
│  · LoggerPlugin  (log start) │
│  · RateLimiterPlugin (Redis) │
└──────────────────┬───────────┘
                   │
              Route matched
                   │
                   ▼
┌──────────────────────────────┐
│  PhasePreUpstream            │  Global + per-route plugins
│  · APIKeyAuthPlugin          │
│  · JWTAuthPlugin             │
│  · (cache middleware checks) │
└──────────────────┬───────────┘
                   │
              Backend call
                   │
                   ▼
┌──────────────────────────────┐
│  PhasePostUpstream           │  Reverse order
│  · (cache store on miss)     │
└──────────────────┬───────────┘
                   │
                   ▼
┌──────────────────────────────┐
│  PhasePreResponse            │  Reverse order
│  · LoggerPlugin (log end)    │
└──────────────────┬───────────┘
                   │
          ResponseCapture.Flush()
                   │
                   ▼
             Client Response
```

### Plugin Priority & Ordering

| Priority | Plugin          | Phase        |
| -------- | --------------- | ------------ |
| 10       | api_key_auth    | PreUpstream  |
| 20       | jwt_auth        | PreUpstream  |
| 50       | rate_limiter    | PreRoute     |
| 100      | logger          | PreRoute + PreResponse |

- Lower priority number = runs **first** in pre phases.
- PostUpstream and PreResponse phases run in **reverse order**.
- Global plugins always run before per-route plugins at the same priority.

---

## Plugin System

### Built-in Plugins

| Plugin         | Phase(s)                   | Description                                     |
| -------------- | -------------------------- | ----------------------------------------------- |
| `logger`       | PreRoute, PreResponse      | Logs request arrival and completion with latency |
| `rate_limiter` | PreRoute                   | Redis token-bucket per-IP rate limiting          |
| `api_key_auth` | PreUpstream                | Validates X-API-Key header on protected routes   |
| `jwt_auth`     | PreUpstream                | Validates Bearer token, forwards claims as headers |

### Writing a Custom External Plugin

Create a Go file that exports a `Factory` symbol of type `plugin.PluginFactory`:

```go
// myplugin/main.go
package main

import (
    "log/slog"
    "hydragate/internal/plugin"
    "hydragate/internal/plugins"
)

type MyPlugin struct {
    plugins.BasePlugin
}

func (p *MyPlugin) Name() string { return "my_plugin" }

func (p *MyPlugin) OnPreRoute(ctx *plugin.PluginContext) error {
    p.Logger.Info("hello from my plugin", "path", ctx.Request.URL.Path)
    return nil
}

var Factory plugin.PluginFactory = func(cfg map[string]interface{}, logger *slog.Logger) (plugin.Plugin, error) {
    p := &MyPlugin{}
    p.Logger = logger
    p.Config = cfg
    return p, nil
}
```

Build it as a shared object:

```bash
go build -buildmode=plugin -o plugins/my_plugin.so ./myplugin
```

Register it in `config.json`:

```json
"plugins": {
  "external_paths": ["./plugins/my_plugin.so"],
  "global": [
    { "name": "my_plugin", "enabled": true, "priority": 200, "on_error": "continue" }
  ],
  "config": {
    "my_plugin": { "some_key": "some_value" }
  }
}
```

### Plugin Config Reference

| Field        | Type   | Default  | Description                              |
| ------------ | ------ | -------- | ---------------------------------------- |
| `name`       | string | required | Plugin identifier (must match `Name()`)  |
| `enabled`    | bool   | `true`   | Enable/disable without removing config   |
| `priority`   | int    | `100`    | Execution order — lower runs first       |
| `timeout_ms` | int    | `5000`   | Per-plugin execution timeout             |
| `on_error`   | string | `"abort"` | `"abort"` or `"continue"` on failure   |

---

## Configuration

HydraGate is fully configured via `config.json`. Below is an annotated example:

```json
{
  "jwt_secret": "your-secret-key",
  "forward_claims": {
    "sub": "X-User-Id",
    "role": "X-User-Role"
  },
  "api_keys": {
    "my-api-key": "my-client-label"
  },
  "rate_limit": {
    "enabled": true,
    "capacity": 100,
    "refill_rate": 10
  },
  "cache": {
    "enabled": false,
    "default_ttl": 300,
    "methods": ["GET"],
    "status_codes": [200]
  },
  "plugins": {
    "external_paths": [],
    "global": [
      { "name": "logger",       "enabled": true, "priority": 100, "on_error": "continue" },
      { "name": "rate_limiter", "enabled": true, "priority": 50,  "on_error": "abort"    }
    ],
    "routes": {
      "api": [
        { "name": "api_key_auth", "enabled": true, "priority": 10, "on_error": "abort" },
        { "name": "jwt_auth",     "enabled": true, "priority": 20, "on_error": "abort" }
      ]
    },
    "config": {
      "rate_limiter": { "capacity": 100, "refill_rate": 10 }
    }
  },
  "routes": [
    {
      "route": "api",
      "target": "http://localhost:9000",
      "protected": true,
      "transform": {
        "add_headers": { "X-Proxied-By": "HydraGate" },
        "remove_headers": ["User-Agent"],
        "path_rewrite": "*"
      },
      "cache": { "enabled": false }
    }
  ]
}
```

---

## Endpoints

| Method | Path      | Description                                      |
| ------ | --------- | ------------------------------------------------ |
| `GET`  | `/health` | Liveness check — returns `"Alive"`               |
| `POST` | `/reload` | Manually trigger config + plugin hot-reload      |
| `ANY`  | `/*`      | All other requests are proxied to backend routes |

### Config Hot-Reload

HydraGate supports zero-downtime hot-reload of the full configuration:

- **Automatic:** `config.json` changes are detected via `fsnotify` and applied within 100ms.
- **Manual:** `curl -X POST http://localhost:8080/reload`

On every reload the following are rebuilt and atomically swapped:
- Route registry
- Plugin executor (with updated plugin entries and per-plugin config)
- Main HTTP handler

---

## Run

**Prerequisites:**
- Go 1.21+
- Redis running on `localhost:6379`

**Start Redis:**

```bash
docker-compose up -d
```

**Run HydraGate:**

```bash
go run ./cmd/server
```

Server starts on `http://localhost:8080`.

---

## Roadmap

### 🟢 Phase 1 — Core Gateway Foundation ✅
- HTTP server, middleware system, reverse proxy, route registry, request logging

### 🟢 Phase 2 — Production Features ✅
- JWT auth, API key auth, rate limiting (Redis), structured logging, request transform, config hot-reload

### 🟢 Phase 3.1 — Caching (Redis) ✅
- Redis caching middleware, per-route/per-path cache control, smart cache key generation

### 🟢 Phase 3.2 — Plugin System ✅
- 4-phase lifecycle (PreRoute / PreUpstream / PostUpstream / PreResponse)
- Plugin registry with external `.so` support
- Per-plugin timeout, priority ordering, `on_error` policy
- Built-in plugins: logger, rate_limiter, jwt_auth, api_key_auth
- Hot-reload of plugin config

### 🔴 Phase 3.3 — Infra & Observability 🔜
- Load balancing (round-robin / weighted)
- Circuit breaker
- Prometheus metrics (`/metrics` endpoint)
- API key management (admin REST API + Redis-backed store)
- Optional dashboard

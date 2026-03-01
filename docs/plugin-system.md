# HydraGate Plugin System

## Overview

A flexible, hot-reloadable plugin system for HydraGate that allows middleware to be plugged in at any part of the request lifecycle.

---

## Critical Design Decisions

These decisions prevent common gateway bugs:

### 1. Factory Pattern (Not Singleton)

**Problem**: Storing plugin instances in registry causes race conditions when multiple goroutines access shared state.

**Solution**: Registry stores `PluginFactory` functions. Executor creates fresh instance per request.

```go
// BAD: Shared instance
plugins map[string]Plugin  // Race condition!

// GOOD: Factory per request
factories map[string]PluginFactory
plugin, _ := factory(config, logger)  // Safe
```

### 2. ResponseCapture Wrapper

**Problem**: Raw `http.ResponseWriter` cannot be inspected or modified after writing.

**Solution**: Wrap with `ResponseCapture` that buffers body and defers writing.

```go
type ResponseCapture struct {
    StatusCode int
    Body       bytes.Buffer
    // ...
}
```

This enables:
- Cache plugin reading response body
- Transform plugin modifying headers
- Logger plugin reading status code

### 3. Request ID Before Plugins

**Problem**: If logger plugin generates request ID, other plugins can't use it.

**Solution**: Inject `X-Request-ID` in middleware BEFORE any plugins run.

### 4. Context with Timeout

**Problem**: `timeout_ms` config is meaningless without actual cancellation.

**Solution**: Pass `context.Context` in PluginContext, wrap with `context.WithTimeout`.

### 5. Reverse Order for Response Phases

**Problem**: Middleware stack unwinding requires reverse execution.

**Solution**: POST_UPSTREAM and PRE_RESPONSE execute in reverse priority order.

```
Request:  A → B → C → Backend
Response: Backend → C → B → A
```

### 6. Single PluginContext Per Request

**Problem**: Recreating context between phases loses metadata.

**Solution**: One `PluginContext` instance flows through all 4 phases.

---

## Design Requirements

Based on discussion:

| Requirement | Decision |
|-------------|----------|
| Plugin scope | Global + Route-specific |
| Hot reload | Yes, via config reload |
| Ordering | Priority number (lower = first) |
| Phases | 4 phases (pre_route, pre_upstream, post_upstream, pre_response) |
| Configuration | Static (config.json) |
| Error handling | Configurable per plugin |
| Custom plugins | Compile-time + .so plugin loader |
| Plugin state | Context-bound only (per-request instances) |
| Metrics | Custom metrics support |
| Dependencies | Optional deps via Metadata |
| Shutdown | Required cleanup hook |
| Logging | Each plugin gets own logger |
| Inter-plugin comms | Metadata map only |
| Config validation | Built-in ValidateConfig() |
| Execution limits | Timeout with context.Context |
| API version mismatch | Fail to load |
| Discovery | Config-based registration |
| Context data | Request, ResponseCapture, Route, Ctx, StartTime, Metadata, Abort |
| Same priority order | Config array order |
| Panic handling | Crash gateway |
| Plugin instantiation | Factory pattern (per-request) |

---

## Plugin Interface

```go
// internal/plugin/types.go

const CurrentAPIVersion = 1

type PluginPhase string

const (
    PhasePreRoute     PluginPhase = "pre_route"      // Before route matching
    PhasePreUpstream  PluginPhase = "pre_upstream"   // After route match, before backend call
    PhasePostUpstream PluginPhase = "post_upstream"  // After backend response received
    PhasePreResponse  PluginPhase = "pre_response"   // Before sending response to client
)

type PluginContext struct {
    Ctx        context.Context           // For timeout/cancellation
    Phase      PluginPhase
    Request    *http.Request
    Response   *ResponseCapture          // Wrapped response for inspection/modification
    Route      *app.RouteConfig          // nil in pre_route phase
    StartTime  time.Time
    Metadata   map[string]interface{}    // Inter-plugin communication
    Abort      bool                      // Set to true to stop request
    AbortCode  int                       // HTTP status code if aborted
    AbortBody  []byte                    // Response body if aborted
}

type Plugin interface {
    // Identity
    Name() string
    APIVersion() int

    // Lifecycle
    Init(config map[string]interface{}, logger *slog.Logger) error
    ValidateConfig(config map[string]interface{}) error
    Shutdown() error

    // Execution (return error to abort with 500)
    OnPreRoute(ctx *PluginContext) error
    OnPreUpstream(ctx *PluginContext) error
    OnPostUpstream(ctx *PluginContext) error
    OnPreResponse(ctx *PluginContext) error

    // Monitoring
    Metrics() map[string]float64
}

// PluginFactory creates a new plugin instance per request
// This prevents race conditions from shared mutable state
type PluginFactory func(config map[string]interface{}, logger *slog.Logger) (Plugin, error)
```

---

## ResponseCapture Wrapper

Plugins cannot safely inspect or modify responses with raw `http.ResponseWriter`.
We wrap it to enable:
- Reading response body (for caching)
- Modifying headers after backend call
- Inspecting status code

```go
// internal/plugin/response.go

type ResponseCapture struct {
    http.ResponseWriter
    StatusCode  int
    Body        bytes.Buffer
    headers     http.Header
    wroteHeader bool
}

func NewResponseCapture(w http.ResponseWriter) *ResponseCapture {
    return &ResponseCapture{
        ResponseWriter: w,
        StatusCode:     200,
        headers:        make(http.Header),
    }
}

func (r *ResponseCapture) Header() http.Header {
    return r.headers
}

func (r *ResponseCapture) WriteHeader(code int) {
    if !r.wroteHeader {
        r.StatusCode = code
        r.wroteHeader = true
    }
}

func (r *ResponseCapture) Write(b []byte) (int, error) {
    if !r.wroteHeader {
        r.WriteHeader(200)
    }
    return r.Body.Write(b)
}

// Flush sends the captured response to the actual client
func (r *ResponseCapture) Flush() {
    for k, vv := range r.headers {
        for _, v := range vv {
            r.ResponseWriter.Header().Add(k, v)
        }
    }
    r.ResponseWriter.WriteHeader(r.StatusCode)
    r.ResponseWriter.Write(r.Body.Bytes())
}

// Bytes returns captured body for inspection
func (r *ResponseCapture) Bytes() []byte {
    return r.Body.Bytes()
}
```

---

## Plugin Configuration

### config.json Structure

```json
{
  "jwt_secret": "...",
  "api_keys": {...},
  "rate_limit": {...},
  "cache": {...},
  "routes": [...],

  "plugins": {
    "external_paths": [
      "./plugins/analytics.so",
      "./plugins/custom_auth.so"
    ],

    "global": [
      {
        "name": "logger",
        "enabled": true,
        "priority": 100,
        "timeout_ms": 5000,
        "on_error": "continue"
      },
      {
        "name": "rate_limiter",
        "enabled": true,
        "priority": 50,
        "timeout_ms": 2000,
        "on_error": "abort"
      }
    ],

    "routes": {
      "api": [
        {
          "name": "cache",
          "enabled": true,
          "priority": 10,
          "timeout_ms": 1000,
          "on_error": "continue"
        }
      ],
      "payments": [
        {
          "name": "audit_logger",
          "enabled": true,
          "priority": 5
        }
      ]
    },

    "config": {
      "logger": {
        "level": "info",
        "include_headers": false
      },
      "rate_limiter": {
        "capacity": 100,
        "refill_rate": 10
      },
      "cache": {
        "default_ttl": 300
      }
    }
  }
}
```

### Plugin Entry Config

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| name | string | yes | - | Plugin identifier |
| enabled | bool | no | true | Enable/disable plugin |
| priority | int | no | 100 | Execution order (lower = first) |
| timeout_ms | int | no | 5000 | Max execution time in ms |
| on_error | string | no | "abort" | "abort" or "continue" |

---

## File Structure

```
internal/
├── plugin/
│   ├── types.go           # Plugin interface, PluginContext, phases
│   ├── factory.go         # PluginFactory type and helpers
│   ├── registry.go        # Plugin factory registration and discovery
│   ├── response.go        # ResponseCapture wrapper
│   ├── executor.go        # Plugin execution engine with timeout
│   └── middleware.go      # HTTP middleware wrapper
│
├── plugins/               # Built-in plugin implementations
│   ├── base.go            # Base plugin with no-op defaults
│   ├── logger.go
│   ├── jwt_auth.go
│   ├── api_key_auth.go
│   ├── rate_limiter.go
│   └── cache.go
```

---

## Plugin Registry

The registry stores **factories**, not instances. This prevents race conditions.

```go
// internal/plugin/registry.go

type PluginRegistry struct {
    factories map[string]PluginFactory
    mu        sync.RWMutex
}

func NewRegistry() *PluginRegistry {
    return &PluginRegistry{
        factories: make(map[string]PluginFactory),
    }
}

// Register a plugin factory
func (r *PluginRegistry) Register(name string, factory PluginFactory) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, exists := r.factories[name]; exists {
        return fmt.Errorf("plugin already registered: %s", name)
    }
    r.factories[name] = factory
    return nil
}

// GetFactory returns factory by name
func (r *PluginRegistry) GetFactory(name string) (PluginFactory, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    f, ok := r.factories[name]
    return f, ok
}

// List all registered plugin names
func (r *PluginRegistry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    names := make([]string, 0, len(r.factories))
    for name := range r.factories {
        names = append(names, name)
    }
    return names
}

// CreateInstance creates a new plugin instance from factory
func (r *PluginRegistry) CreateInstance(name string, config map[string]interface{}) (Plugin, error) {
    factory, ok := r.GetFactory(name)
    if !ok {
        return nil, fmt.Errorf("plugin not found: %s", name)
    }
    
    logger := slog.Default().With("plugin", name)
    return factory(config, logger)
}

// LoadExternal loads .so plugins from paths
func (r *PluginRegistry) LoadExternal(paths []string) error {
    for _, path := range paths {
        plug, err := plugin.Open(path)
        if err != nil {
            return fmt.Errorf("failed to open plugin %s: %w", path, err)
        }
        
        sym, err := plug.Lookup("Factory")
        if err != nil {
            return fmt.Errorf("plugin %s missing Factory symbol: %w", path, err)
        }
        
        factory, ok := sym.(*PluginFactory)
        if !ok {
            return fmt.Errorf("plugin %s Factory has wrong type", path)
        }
        
        // Create temp instance to get name and validate API version
        tempPlugin, err := (*factory)(nil, nil)
        if err != nil {
            return fmt.Errorf("plugin %s factory failed: %w", path, err)
        }
        
        if tempPlugin.APIVersion() != CurrentAPIVersion {
            return fmt.Errorf("plugin %s has incompatible API version %d (expected %d)",
                path, tempPlugin.APIVersion(), CurrentAPIVersion)
        }
        
        r.Register(tempPlugin.Name(), *factory)
    }
    return nil
}
```

---

## Plugin Executor

```go
// internal/plugin/executor.go

type PluginEntry struct {
    Name      string
    Enabled   bool
    Priority  int
    TimeoutMs int
    OnError   string  // "abort" or "continue"
}

type PluginExecutor struct {
    registry     *PluginRegistry
    globalChain  []PluginEntry
    routeChains  map[string][]PluginEntry
    configs      map[string]map[string]interface{}
    chainTimeout time.Duration
    mu           sync.RWMutex
}

func NewExecutor(registry *PluginRegistry) *PluginExecutor

// Update configuration (hot reload)
func (e *PluginExecutor) UpdateConfig(cfg PluginsConfig) error

// Execute a single plugin with timeout
func (e *PluginExecutor) executeWithTimeout(
    p Plugin,
    ctx *PluginContext,
    phase PluginPhase,
    timeout time.Duration,
) error {
    timeoutCtx, cancel := context.WithTimeout(ctx.Ctx, timeout)
    defer cancel()
    
    // Update context with timeout
    originalCtx := ctx.Ctx
    ctx.Ctx = timeoutCtx
    defer func() { ctx.Ctx = originalCtx }()
    
    done := make(chan error, 1)
    go func() {
        var err error
        switch phase {
        case PhasePreRoute:
            err = p.OnPreRoute(ctx)
        case PhasePreUpstream:
            err = p.OnPreUpstream(ctx)
        case PhasePostUpstream:
            err = p.OnPostUpstream(ctx)
        case PhasePreResponse:
            err = p.OnPreResponse(ctx)
        }
        done <- err
    }()
    
    select {
    case err := <-done:
        return err
    case <-timeoutCtx.Done():
        return fmt.Errorf("plugin %s timed out after %v", p.Name(), timeout)
    }
}

// Execute plugins for a phase
func (e *PluginExecutor) Execute(ctx *PluginContext, routePrefix string, phase PluginPhase) error {
    e.mu.RLock()
    chain := e.buildChain(routePrefix, phase)
    e.mu.RUnlock()
    
    for _, entry := range chain {
        if !entry.Enabled {
            continue
        }
        
        plugin, err := e.registry.CreateInstance(entry.Name, e.configs[entry.Name])
        if err != nil {
            if entry.OnError == "continue" {
                slog.Error("plugin init failed", "plugin", entry.Name, "error", err)
                continue
            }
            return err
        }
        
        timeout := time.Duration(entry.TimeoutMs) * time.Millisecond
        if timeout == 0 {
            timeout = 5 * time.Second // default
        }
        
        err = e.executeWithTimeout(plugin, ctx, phase, timeout)
        if err != nil {
            if entry.OnError == "continue" {
                slog.Error("plugin execution failed",
                    "plugin", entry.Name,
                    "phase", phase,
                    "error", err,
                    "request_id", ctx.Metadata["request_id"],
                )
                continue
            }
            return err
        }
        
        if ctx.Abort {
            return nil // Stop chain, but not an error
        }
    }
    return nil
}

// buildChain merges global + route plugins, sorted by priority
func (e *PluginExecutor) buildChain(routePrefix string, phase PluginPhase) []PluginEntry {
    var chain []PluginEntry
    
    // Add global plugins
    chain = append(chain, e.globalChain...)
    
    // Add route-specific plugins
    if routePlugins, ok := e.routeChains[routePrefix]; ok {
        chain = append(chain, routePlugins...)
    }
    
    // Sort by priority (stable sort preserves config order for same priority)
    sort.SliceStable(chain, func(i, j int) bool {
        return chain[i].Priority < chain[j].Priority
    })
    
    // Reverse for response phases (stack unwinding)
    if phase == PhasePostUpstream || phase == PhasePreResponse {
        for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
            chain[i], chain[j] = chain[j], chain[i]
        }
    }
    
    return chain
}

// Build HTTP middleware
func (e *PluginExecutor) Middleware() func(http.Handler) http.Handler
```

---

## HTTP Middleware Integration

The middleware wrapper integrates the plugin system with the HTTP handler chain.
Request ID is injected BEFORE any plugins run.

```go
// internal/plugin/middleware.go

func (e *PluginExecutor) Middleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. Inject request ID BEFORE plugins
            requestID := uuid.New().String()
            r.Header.Set("X-Request-ID", requestID)
            
            // 2. Create plugin context (single context for entire request)
            respCapture := NewResponseCapture(w)
            ctx := &PluginContext{
                Ctx:       r.Context(),
                Request:   r,
                Response:  respCapture,
                StartTime: time.Now(),
                Metadata:  map[string]interface{}{"request_id": requestID},
            }
            
            // 3. PRE_ROUTE phase (global only, route not matched yet)
            if err := e.Execute(ctx, "", PhasePreRoute); err != nil {
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
                return
            }
            if ctx.Abort {
                w.WriteHeader(ctx.AbortCode)
                w.Write(ctx.AbortBody)
                return
            }
            
            // 4. Route matching happens in proxy.Forward
            //    PRE_UPSTREAM called after route is known
            //    (executor needs route prefix from proxy)
            
            // 5. Store context for later phases
            r = r.WithContext(context.WithValue(r.Context(), "plugin_ctx", ctx))
            
            // 6. Call next handler (proxy)
            next.ServeHTTP(respCapture, r)
            
            // 7. POST_UPSTREAM and PRE_RESPONSE phases
            //    (called from proxy after backend response)
        })
    }
}
```

---

## Execution Flow

```
Client Request
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  REQUEST ID INJECTION (before any plugins)                   │
│  X-Request-ID: uuid                                          │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────┐
│  PRE_ROUTE      │  Global plugins only (route not matched yet)
│  - Logger start │
│  - Rate limiter │
└────────┬────────┘
         │
         ▼
   Route Matching
         │
         ▼
┌─────────────────┐
│  PRE_UPSTREAM   │  Global + Route plugins
│  - JWT Auth     │
│  - API Key Auth │
│  - Cache lookup │
│  - Transform    │
└────────┬────────┘
         │
         ▼
   Backend Request
         │
         ▼
┌─────────────────┐
│  POST_UPSTREAM  │  Global + Route plugins (reverse order)
│  - Cache store  │
│  - Transform    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  PRE_RESPONSE   │  Global + Route plugins (reverse order)
│  - Logger end   │
│  - Metrics      │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│  RESPONSE FLUSH                                              │
│  ResponseCapture.Flush() → send to client                    │
└─────────────────────────────────────────────────────────────┘
```

### Priority & Order Rules

1. Sort all plugins by priority (ascending)
2. Same priority: use config array order
3. Global plugins execute before route plugins in each phase
4. POST_UPSTREAM and PRE_RESPONSE phases execute in reverse order

---

## Built-in Plugins

All built-in plugins are registered as factories.

### Base Plugin (Embed for Defaults)

```go
// internal/plugins/base.go

// BasePlugin provides no-op defaults for all methods
// Embed this to only implement the phases you need
type BasePlugin struct {
    logger *slog.Logger
    config map[string]interface{}
}

func (p *BasePlugin) Name() string                                    { return "base" }
func (p *BasePlugin) APIVersion() int                                 { return CurrentAPIVersion }
func (p *BasePlugin) Init(cfg map[string]interface{}, l *slog.Logger) error {
    p.config = cfg
    p.logger = l
    return nil
}
func (p *BasePlugin) ValidateConfig(cfg map[string]interface{}) error { return nil }
func (p *BasePlugin) Shutdown() error                                 { return nil }
func (p *BasePlugin) OnPreRoute(ctx *PluginContext) error             { return nil }
func (p *BasePlugin) OnPreUpstream(ctx *PluginContext) error          { return nil }
func (p *BasePlugin) OnPostUpstream(ctx *PluginContext) error         { return nil }
func (p *BasePlugin) OnPreResponse(ctx *PluginContext) error          { return nil }
func (p *BasePlugin) Metrics() map[string]float64                     { return nil }
```

### 1. Logger Plugin

```go
// internal/plugins/logger.go

type LoggerPlugin struct {
    BasePlugin
}

func LoggerFactory(cfg map[string]interface{}, logger *slog.Logger) (Plugin, error) {
    p := &LoggerPlugin{}
    p.logger = logger
    p.config = cfg
    return p, nil
}

func (p *LoggerPlugin) Name() string { return "logger" }

func (p *LoggerPlugin) OnPreRoute(ctx *PluginContext) error {
    // Start time already in ctx.StartTime
    // Request ID already in ctx.Metadata["request_id"]
    return nil
}

func (p *LoggerPlugin) OnPreResponse(ctx *PluginContext) error {
    duration := time.Since(ctx.StartTime)
    p.logger.Info("request completed",
        "request_id", ctx.Metadata["request_id"],
        "method", ctx.Request.Method,
        "path", ctx.Request.URL.Path,
        "status", ctx.Response.StatusCode,
        "duration_ms", duration.Milliseconds(),
        "remote_addr", ctx.Request.RemoteAddr,
    )
    return nil
}
```

### 2. Rate Limiter Plugin

```go
// internal/plugins/rate_limiter.go

type RateLimiterPlugin struct {
    BasePlugin
    rdb      *redis.Client
    capacity int
    refill   int
}

func RateLimiterFactory(cfg map[string]interface{}, logger *slog.Logger) (Plugin, error) {
    p := &RateLimiterPlugin{}
    p.logger = logger
    p.config = cfg
    
    // Extract config
    if cap, ok := cfg["capacity"].(float64); ok {
        p.capacity = int(cap)
    }
    if refill, ok := cfg["refill_rate"].(float64); ok {
        p.refill = int(refill)
    }
    // Redis client injected separately via Init or from config
    
    return p, nil
}

func (p *RateLimiterPlugin) Name() string { return "rate_limiter" }

func (p *RateLimiterPlugin) OnPreRoute(ctx *PluginContext) error {
    // Token bucket check using Redis
    clientIP := ctx.Request.RemoteAddr
    
    allowed, err := p.checkLimit(ctx.Ctx, clientIP)
    if err != nil {
        p.logger.Error("rate limit check failed", "error", err)
        return err
    }
    
    if !allowed {
        ctx.Abort = true
        ctx.AbortCode = http.StatusTooManyRequests
        ctx.AbortBody = []byte(`{"error": "rate limit exceeded"}`)
    }
    return nil
}

func (p *RateLimiterPlugin) Metrics() map[string]float64 {
    return map[string]float64{
        "hydragate_plugin_rate_limiter_blocked_total": 0, // track in plugin state
        "hydragate_plugin_rate_limiter_allowed_total": 0,
    }
}
```

### 3. JWT Auth Plugin

- Phase: PRE_UPSTREAM
- Validates Bearer token
- Forwards claims as headers
- Aborts with 401 if invalid

### 4. API Key Auth Plugin

- Phase: PRE_UPSTREAM
- Checks X-API-Key header
- Sets X-Authenticated-By header
- Aborts with 401 if invalid

### 5. Cache Plugin

```go
// internal/plugins/cache.go

type CachePlugin struct {
    BasePlugin
    rdb *redis.Client
    ttl time.Duration
}

func CacheFactory(cfg map[string]interface{}, logger *slog.Logger) (Plugin, error) {
    p := &CachePlugin{}
    p.logger = logger
    p.config = cfg
    
    if ttl, ok := cfg["default_ttl"].(float64); ok {
        p.ttl = time.Duration(ttl) * time.Second
    }
    return p, nil
}

func (p *CachePlugin) Name() string { return "cache" }

func (p *CachePlugin) OnPreUpstream(ctx *PluginContext) error {
    // Check cache for GET requests
    if ctx.Request.Method != http.MethodGet {
        return nil
    }
    
    key := p.buildCacheKey(ctx.Request)
    cached, err := p.rdb.Get(ctx.Ctx, key).Bytes()
    if err == nil {
        // Cache HIT - write response and abort (skip backend)
        ctx.Response.WriteHeader(http.StatusOK)
        ctx.Response.Write(cached)
        ctx.Abort = true
        ctx.Metadata["cache_hit"] = true
        return nil
    }
    ctx.Metadata["cache_key"] = key
    return nil
}

func (p *CachePlugin) OnPostUpstream(ctx *PluginContext) error {
    // Store in cache if cacheable
    if ctx.Response.StatusCode != http.StatusOK {
        return nil
    }
    if _, hit := ctx.Metadata["cache_hit"]; hit {
        return nil // Already from cache
    }
    
    key, ok := ctx.Metadata["cache_key"].(string)
    if !ok {
        return nil
    }
    
    body := ctx.Response.Bytes()
    p.rdb.Set(ctx.Ctx, key, body, p.ttl)
    return nil
}
```

---

## External Plugin Development

### Creating a .so Plugin

External plugins must export a `Factory` variable of type `PluginFactory`.

```go
// plugins/analytics/main.go
package main

import (
    "hydragate/internal/plugin"
    "log/slog"
)

type AnalyticsPlugin struct {
    plugin.BasePlugin
    endpoint string
}

func (p *AnalyticsPlugin) Name() string      { return "analytics" }
func (p *AnalyticsPlugin) APIVersion() int   { return plugin.CurrentAPIVersion }

func (p *AnalyticsPlugin) Init(cfg map[string]interface{}, logger *slog.Logger) error {
    p.BasePlugin.Init(cfg, logger)
    if ep, ok := cfg["endpoint"].(string); ok {
        p.endpoint = ep
    }
    return nil
}

func (p *AnalyticsPlugin) ValidateConfig(cfg map[string]interface{}) error {
    if _, ok := cfg["endpoint"]; !ok {
        return fmt.Errorf("analytics plugin requires 'endpoint' config")
    }
    return nil
}

func (p *AnalyticsPlugin) OnPreResponse(ctx *plugin.PluginContext) error {
    // Send analytics data
    go p.sendAnalytics(ctx)
    return nil
}

func (p *AnalyticsPlugin) Shutdown() error {
    // Flush pending analytics
    return nil
}

func (p *AnalyticsPlugin) Metrics() map[string]float64 {
    return map[string]float64{
        "hydragate_plugin_analytics_events_sent": 1234,
    }
}

// REQUIRED: Export factory function
var Factory plugin.PluginFactory = func(cfg map[string]interface{}, logger *slog.Logger) (plugin.Plugin, error) {
    p := &AnalyticsPlugin{}
    if err := p.Init(cfg, logger); err != nil {
        return nil, err
    }
    return p, nil
}
```

### Building External Plugin

```bash
go build -buildmode=plugin -o plugins/analytics.so plugins/analytics/main.go
```

### Important Notes for External Plugins

1. **Go Version**: Plugin must be built with exact same Go version as gateway
2. **Dependencies**: All shared dependencies must have same versions
3. **API Version**: Plugin's `APIVersion()` must match `CurrentAPIVersion`
4. **Factory Export**: Must export `var Factory plugin.PluginFactory`

---

## Hot Reload

When `/reload` endpoint is called:

1. Load new config.json
2. Validate all plugin configs
3. Call `executor.UpdateConfig(newPluginsConfig)`
4. Active requests complete with old config
5. New requests use new config

---

## Error Handling

| on_error | Behavior |
|----------|----------|
| "abort" | Return error response, stop processing |
| "continue" | Log error, continue to next plugin |

Plugin errors logged with:
- Plugin name
- Phase
- Error message
- Request ID

---

## Metrics

Each plugin can expose custom metrics via `Metrics()`.
Metrics should be namespaced to avoid collisions.

### Naming Convention

```
hydragate_plugin_<plugin_name>_<metric_name>
```

### Example

```go
func (p *RateLimiterPlugin) Metrics() map[string]float64 {
    return map[string]float64{
        "hydragate_plugin_rate_limiter_blocked_total": p.blockedCount,
        "hydragate_plugin_rate_limiter_allowed_total": p.allowedCount,
    }
}
```

### Gateway Metrics Endpoint

Gateway exposes `/metrics` endpoint (Prometheus format).
Aggregates metrics from all plugins.

```go
// cmd/server/main.go

http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    metrics := make(map[string]float64)
    
    for _, name := range registry.List() {
        plugin, _ := registry.CreateInstance(name, configs[name])
        for k, v := range plugin.Metrics() {
            metrics[k] = v
        }
    }
    
    // Format as Prometheus
    for name, value := range metrics {
        fmt.Fprintf(w, "%s %f\n", name, value)
    }
})
```

---

## Implementation Phases

### Phase 1: Core Framework
- [ ] Plugin interface and types (`internal/plugin/types.go`)
- [ ] PluginFactory type (`internal/plugin/factory.go`)
- [ ] ResponseCapture wrapper (`internal/plugin/response.go`)
- [ ] Plugin registry with factories (`internal/plugin/registry.go`)
- [ ] Plugin executor with timeout (`internal/plugin/executor.go`)
- [ ] HTTP middleware integration (`internal/plugin/middleware.go`)
- [ ] Request ID injection

### Phase 2: Built-in Plugins
- [ ] Base plugin with defaults (`internal/plugins/base.go`)
- [ ] Migrate Logger to plugin
- [ ] Migrate JWT Auth to plugin
- [ ] Migrate API Key Auth to plugin
- [ ] Migrate Rate Limiter to plugin
- [ ] Migrate Cache to plugin

### Phase 3: External Plugins
- [ ] .so plugin loader in registry
- [ ] API version validation
- [ ] Example external plugin with build instructions

### Phase 4: Polish
- [ ] Metrics endpoint with namespacing
- [ ] Hot reload testing
- [ ] Documentation and examples

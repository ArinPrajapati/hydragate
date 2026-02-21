# JWT Authentication — Architecture Doc

## Overview

HydraGate validates JWT tokens on protected routes, injects configurable claims as headers, and forwards requests to backend services. The gateway **never generates tokens** — that's the job of a separate auth service.

```
Client → [GET /api/users + Bearer token]
   │
   ▼
HydraGate
   ├── Logger middleware   → logs request metadata
   ├── JWTAuth middleware  → validates token, injects headers
   └── Proxy (Forward)     → forwards to backend with injected headers
         │
         ▼
   Backend Service (receives X-User-Id, X-User-Role headers)
```

---

## Config-Driven Design

Everything is controlled from `config.json`:

```json
{
  "jwt_secret": "...",
  "forward_claims": {
    "sub": "X-User-Id",
    "role": "X-User-Role"
  },
  "routes": [
    { "route": "api", "target": "http://localhost:9000", "protected": true },
    { "route": "date", "target": "http://localhost:9000", "protected": false }
  ]
}
```

| Field                | Purpose                                                                 |
| -------------------- | ----------------------------------------------------------------------- |
| `jwt_secret`         | HMAC-SHA256 signing key for token validation                            |
| `forward_claims`     | Maps JWT claim names → HTTP header names injected into proxied requests |
| `routes[].protected` | Whether a route requires a valid JWT                                    |

---

## Request Lifecycle (Protected Route)

```
1. Client sends:  GET /api/users
                   Authorization: Bearer eyJhbG...

2. "/" handler matches → middleware.Chain executes:
   a. Logger middleware      → records start time, generates request_id
   b. JWTAuth middleware     → extracts path segment "api"
                              → checks ProtectedRoutes["api"] == true
                              → parses "Bearer <token>" from Authorization header
                              → calls auth.ValidateToken(token, secret)
                              → on success: loops forward_claims config
                                 claims["sub"] → r.Header.Set("X-User-Id", "admin")
                                 claims["role"] → r.Header.Set("X-User-Role", "admin")
                              → calls next handler
   c. proxy.Forward          → extracts route "api" → looks up target
                              → builds URL: http://localhost:9000/users
                              → clones headers (now includes X-User-Id, X-User-Role)
                              → sends request to backend
                              → copies response back to client
   d. Logger middleware      → logs: status, method, path, latency_ms, request_id

3. Client receives backend response
```

---

## Request Lifecycle (Unprotected Route)

```
1. Client sends:  GET /date/now  (no Authorization header)

2. JWTAuth middleware → checks ProtectedRoutes["date"] == false → skips auth

3. proxy.Forward → forwards directly to backend

4. Client receives response — no auth required
```

---

## File Structure

```
internal/
├── app/
│   └── app.go            # GatewayConfig, RouteConfig structs
├── auth/
│   ├── auth.go           # GenerateToken(), ValidateToken()
│   └── handler.go        # LoginHandler() — demo endpoint for testing
├── config/
│   ├── loader.go         # LoadConfig() → returns *GatewayConfig
│   └── prase.go          # ParseConfig() — reads & unmarshals JSON
├── middleware/
│   ├── Chain.go          # Chain() — composes middlewares
│   ├── Logger.go         # Logger — structured JSON logging via slog
│   └── JWTAuth.go        # JWTAuth() — token validation + header injection
└── proxy/
    ├── forward.go        # Forward() — reverse proxy handler
    └── registry.go       # Registry — route table with Protected flag
```

---

## Key Design Decisions

### 1. MapClaims for Dynamic Header Injection

We use `jwt.MapClaims` (a `map[string]interface{}`) instead of a typed claims struct. This allows the `forward_claims` config to work with **any** claim name without code changes:

```go
for claimName, headerName := range cfg.ForwardClaims {
    if value, ok := claims[claimName]; ok {
        r.Header.Set(headerName, fmt.Sprintf("%v", value))
    }
}
```

Adding `"email": "X-User-Email"` to config just works — no recompilation needed.

### 2. Gateway Doesn't Generate Tokens

The gateway's job is **validation only**. Token generation (`auth.go` + `handler.go`) exists as a testing convenience in `test/server/start.go`. In production, a separate auth service handles login.

### 3. Per-Route Protection via Registry

The `Registry.ProtectedRoutes()` method builds a `map[string]bool` lookup that the JWT middleware uses. This keeps the middleware fast — O(1) lookup per request.

### 4. Middleware Chain Order

```go
http.Handle("/", middleware.Chain(
    handler,          // innermost — runs last
    middleware.Logger, // runs first — wraps everything
    jwtAuth,          // runs second — blocks unauthorized requests
))
```

Chain wraps inside-out: `Logger(JWTAuth(handler))`. Logger captures timing for the entire request including auth.

---

## Dependencies

| Package                        | Purpose                  |
| ------------------------------ | ------------------------ |
| `log/slog` (stdlib)            | Structured JSON logging  |
| `github.com/golang-jwt/jwt/v5` | JWT signing & validation |
| `github.com/google/uuid`       | Request ID generation    |

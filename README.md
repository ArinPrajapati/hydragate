# HydraGate

A production-grade API Gateway built in Go.

HydraGate sits between clients and backend services, acting as the single entry point for all traffic.

```
Client → HydraGate → Backend Services
```

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

## Project Structure

```
hydragate/
├── cmd/
│   └── server/
│       └── main.go          # Entry point, route config
├── internal/
│   ├── middleware/
│   │   ├── Chain.go         # Middleware chaining
│   │   └── Logger.go        # Request logger (method, status, latency, UUID)
│   └── proxy/
│       ├── registry.go      # Route registry (prefix → target URL)
│       └── forward.go       # Reverse proxy logic
└── product.md               # Full feature roadmap
```

---

## How It Works

1. **Server** starts on `:8080` and registers routes.
2. Every request passes through the **middleware chain** (Logger → handler).
3. The **proxy** resolves the request's path prefix against the route registry and forwards it to the correct backend.
4. **Logger** captures method, status code, latency, and a unique request ID (UUID) for every request.

### Example routing config (in `main.go`)

```go
reg := proxy.NewRegistry()
reg.AddRoute("api", "http://localhost:9000")
// GET /api/users → http://localhost:9000/users
```

### Health check

```
GET /health → "Alive"
```

---

## Run

```bash
go run ./cmd/server
```

---

## Roadmap

### 🟡 Phase 2 — Production Features

- JWT authentication
- API key system
- Rate limiting (Redis)
- Structured logging
- Request/response transform
- Config hot-reload

### 🔴 Phase 3 — Pro / Infra Level

- Caching (Redis)
- Plugin system
- Load balancing
- Circuit breaker
- Prometheus metrics
- Docker setup
- Optional dashboard

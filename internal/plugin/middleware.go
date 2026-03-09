package plugin

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ContextKey string

const PluginCtxKey ContextKey = "plugin_ctx"

// Middleware returns an HTTP middleware that drives the full 4-phase plugin
// lifecycle for every request:
//
//	PhasePreRoute     — global plugins before route matching
//	PhasePreUpstream  — global + route plugins after route is known, before proxy
//	PhasePostUpstream — global + route plugins after backend responds (reverse order)
//	PhasePreResponse  — global + route plugins just before flushing to client (reverse order)
//
// The route prefix is resolved from the request URL by the routePrefix function
// so that the executor can load the correct per-route plugin chain for the
// pre/post upstream phases.
func (e *PluginExecutor) Middleware(routePrefix func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ── 1. Inject a unique request ID before any plugin runs ──────────
			requestID := uuid.New().String()
			r.Header.Set("X-Request-ID", requestID)

			// ── 2. Create the single PluginContext that travels the whole lifecycle
			respCapture := NewResponseCapture(w)
			ctx := &PluginContext{
				Ctx:       r.Context(),
				Phase:     PhasePreRoute,
				Request:   r,
				Response:  respCapture,
				StartTime: time.Now(),
				Metadata:  map[string]interface{}{"request_id": requestID},
			}

			// ── 3. PRE_ROUTE — global-only (route not matched yet) ────────────
			ctx.Phase = PhasePreRoute
			if err := e.Execute(ctx, "", PhasePreRoute); err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if ctx.Abort {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(ctx.AbortCode)
				w.Write(ctx.AbortBody)
				return
			}

			// ── 4. Resolve route prefix for phase-aware execution ─────────────
			prefix := routePrefix(r)

			// Store the plugin context in the request context so downstream
			// handlers (e.g. the proxy) can read it if needed.
			r = r.WithContext(context.WithValue(r.Context(), PluginCtxKey, ctx))

			// ── 5. PRE_UPSTREAM — global + route plugins ──────────────────────
			ctx.Phase = PhasePreUpstream
			if err := e.Execute(ctx, prefix, PhasePreUpstream); err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if ctx.Abort {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(ctx.AbortCode)
				w.Write(ctx.AbortBody)
				return
			}

			// ── 6. Call next handler (the reverse proxy) ──────────────────────
			// We pass respCapture so the response is buffered and we can run
			// post-upstream phases before flushing bytes to the real client.
			next.ServeHTTP(respCapture, r)

			// ── 7. POST_UPSTREAM — reverse order ──────────────────────────────
			ctx.Phase = PhasePostUpstream
			if err := e.Execute(ctx, prefix, PhasePostUpstream); err != nil {
				// Log but don't abort — the response has already been written
				// to the capture buffer; flushing is still possible.
				ctx.Response.StatusCode = http.StatusInternalServerError
			}

			// ── 8. PRE_RESPONSE — reverse order ───────────────────────────────
			ctx.Phase = PhasePreResponse
			if err := e.Execute(ctx, prefix, PhasePreResponse); err != nil {
				ctx.Response.StatusCode = http.StatusInternalServerError
			}

			// ── 9. Flush captured response to the real client ─────────────────
			respCapture.Flush()
		})
	}
}

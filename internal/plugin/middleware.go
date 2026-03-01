package plugin

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ContextKey string

const PluginCtxKey ContextKey = "plugin_ctx"

func (e *PluginExecutor) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.New().String()
			r.Header.Set("X-Request-ID", requestID)

			respCapture := NewResponseCapture(w)
			ctx := &PluginContext{
				Ctx:       r.Context(),
				Request:   r,
				Response:  respCapture,
				StartTime: time.Now(),
				Metadata:  map[string]interface{}{"request_id": requestID},
			}

			if err := e.Execute(ctx, "", PhasePreRoute); err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if ctx.Abort {
				w.WriteHeader(ctx.AbortCode)
				w.Write(ctx.AbortBody)
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), PluginCtxKey, ctx))

			next.ServeHTTP(respCapture, r)
		})
	}
}

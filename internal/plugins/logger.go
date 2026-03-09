package plugins

import (
	"log/slog"
	"time"

	"hydragate/internal/plugin"
)

// LoggerPlugin logs request arrival and completion (method, path, status, latency).
// It hooks into two phases:
//   - OnPreRoute  → records start time and request ID (already in ctx, but we log arrival)
//   - OnPreResponse → logs the completed request with status and duration
type LoggerPlugin struct {
	BasePlugin
}

func (p *LoggerPlugin) Name() string { return "logger" }

func LoggerFactory(cfg map[string]interface{}, logger *slog.Logger) (plugin.Plugin, error) {
	p := &LoggerPlugin{}
	p.Logger = logger
	p.Config = cfg
	return p, nil
}

// OnPreRoute fires before route matching — log that the request arrived.
func (p *LoggerPlugin) OnPreRoute(ctx *plugin.PluginContext) error {
	p.Logger.Info("request received",
		"request_id", ctx.Metadata["request_id"],
		"method", ctx.Request.Method,
		"path", ctx.Request.URL.Path,
		"remote_addr", ctx.Request.RemoteAddr,
	)
	return nil
}

// OnPreResponse fires after the backend has responded — log the final outcome.
func (p *LoggerPlugin) OnPreResponse(ctx *plugin.PluginContext) error {
	duration := time.Since(ctx.StartTime)

	p.Logger.Info("request completed",
		"request_id", ctx.Metadata["request_id"],
		"method", ctx.Request.Method,
		"path", ctx.Request.URL.Path,
		"status", ctx.Response.StatusCode,
		"latency_ms", duration.Milliseconds(),
		"remote_addr", ctx.Request.RemoteAddr,
	)
	return nil
}

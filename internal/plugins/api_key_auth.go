package plugins

import (
	"log/slog"
	"net/http"

	"hydragate/internal/plugin"
	"hydragate/internal/urlpath"
)

// APIKeyAuthPlugin validates the X-API-Key header on protected routes.
// It runs in PhasePreUpstream — before the request is forwarded to the backend.
// On success it sets X-Authenticated-By: api-key and X-API-Key-Name: <label>
// so that the JWT plugin (and upstream services) know the request is already
// authenticated via an API key.
type APIKeyAuthPlugin struct {
	BasePlugin
	keys            map[string]string // api_key → label
	protectedRoutes map[string]bool
}

func (p *APIKeyAuthPlugin) Name() string { return "api_key_auth" }

// NewAPIKeyAuthFactory returns a PluginFactory closed over live config accessors.
// Both keys and protectedRoutes are fetched at factory-call time so hot-reload
// of config.json is automatically reflected in every new request.
func NewAPIKeyAuthFactory(
	getKeys func() map[string]string,
	getProtectedRoutes func() map[string]bool,
) plugin.PluginFactory {
	return func(cfg map[string]interface{}, logger *slog.Logger) (plugin.Plugin, error) {
		p := &APIKeyAuthPlugin{
			keys:            getKeys(),
			protectedRoutes: getProtectedRoutes(),
		}
		p.Logger = logger
		p.Config = cfg
		return p, nil
	}
}

// OnPreUpstream fires after route matching but before the request is proxied.
// API-key auth intentionally runs before JWT auth (lower priority number) so
// that a valid API key short-circuits the JWT check entirely.
func (p *APIKeyAuthPlugin) OnPreUpstream(ctx *plugin.PluginContext) error {
	parsed, err := urlpath.Parse(ctx.Request.URL.Path)
	if err != nil {
		ctx.Abort = true
		ctx.AbortCode = http.StatusBadRequest
		ctx.AbortBody = jsonError("invalid request path")
		return nil
	}

	// Not a protected route — nothing to do.
	if !p.protectedRoutes[parsed.Prefix] {
		return nil
	}

	apiKey := ctx.Request.Header.Get("X-API-Key")

	// No API key header present — let JWT auth handle it downstream.
	if apiKey == "" {
		return nil
	}

	label, valid := p.keys[apiKey]
	if !valid {
		p.Logger.Warn("invalid api key",
			"path", ctx.Request.URL.Path,
			"remote_addr", ctx.Request.RemoteAddr,
			"request_id", ctx.Metadata["request_id"],
		)
		ctx.Abort = true
		ctx.AbortCode = http.StatusUnauthorized
		ctx.AbortBody = jsonError("invalid api key")
		return nil
	}

	// Mark request as API-key authenticated so JWT plugin skips its check.
	ctx.Request.Header.Set("X-Authenticated-By", "api-key")
	ctx.Request.Header.Set("X-API-Key-Name", label)

	p.Logger.Debug("api key auth passed",
		"path", ctx.Request.URL.Path,
		"key_name", label,
		"request_id", ctx.Metadata["request_id"],
	)

	return nil
}

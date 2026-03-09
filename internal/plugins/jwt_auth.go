package plugins

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"hydragate/internal/auth"
	"hydragate/internal/plugin"
	"hydragate/internal/urlpath"
)

// JWTAuthPlugin validates Bearer tokens on protected routes and forwards
// configured JWT claims as request headers to the upstream service.
// It runs in PhasePreUpstream so route matching has already happened.
type JWTAuthPlugin struct {
	BasePlugin
	secret          string
	forwardClaims   map[string]string // claim name → header name
	protectedRoutes map[string]bool
}

func (p *JWTAuthPlugin) Name() string { return "jwt_auth" }

// NewJWTAuthFactory returns a PluginFactory closed over the dynamic gateway state.
// secret, forwardClaims, and protectedRoutes are read from the live config at
// factory-call time (i.e. per request rebuild), so hot-reload is respected.
func NewJWTAuthFactory(
	getSecret func() string,
	getForwardClaims func() map[string]string,
	getProtectedRoutes func() map[string]bool,
) plugin.PluginFactory {
	return func(cfg map[string]interface{}, logger *slog.Logger) (plugin.Plugin, error) {
		p := &JWTAuthPlugin{
			secret:          getSecret(),
			forwardClaims:   getForwardClaims(),
			protectedRoutes: getProtectedRoutes(),
		}
		p.Logger = logger
		p.Config = cfg
		return p, nil
	}
}

// OnPreUpstream fires after route matching but before the request is forwarded.
// For protected routes it validates the Bearer token (unless the request was
// already authenticated by the API-key plugin) and injects claim headers.
func (p *JWTAuthPlugin) OnPreUpstream(ctx *plugin.PluginContext) error {
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

	// API-key plugin already authenticated this request — skip JWT check.
	if ctx.Request.Header.Get("X-Authenticated-By") == "api-key" {
		return nil
	}

	authHeader := ctx.Request.Header.Get("Authorization")
	if authHeader == "" {
		p.Logger.Warn("missing authorization header",
			"path", ctx.Request.URL.Path,
			"request_id", ctx.Metadata["request_id"],
		)
		ctx.Abort = true
		ctx.AbortCode = http.StatusUnauthorized
		ctx.AbortBody = jsonError("missing authorization header")
		return nil
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		ctx.Abort = true
		ctx.AbortCode = http.StatusUnauthorized
		ctx.AbortBody = jsonError("invalid authorization format, expected: Bearer <token>")
		return nil
	}

	claims, err := auth.ValidateToken(parts[1], p.secret)
	if err != nil {
		p.Logger.Warn("jwt validation failed",
			"error", err.Error(),
			"path", ctx.Request.URL.Path,
			"remote_addr", ctx.Request.RemoteAddr,
			"request_id", ctx.Metadata["request_id"],
		)
		ctx.Abort = true
		ctx.AbortCode = http.StatusUnauthorized
		ctx.AbortBody = jsonError("invalid or expired token")
		return nil
	}

	// Forward configured claims as request headers to the upstream service.
	for claimName, headerName := range p.forwardClaims {
		if value, ok := claims[claimName]; ok {
			ctx.Request.Header.Set(headerName, fmt.Sprintf("%v", value))
		}
	}

	p.Logger.Debug("jwt auth passed",
		"path", ctx.Request.URL.Path,
		"claims_forwarded", len(p.forwardClaims),
		"request_id", ctx.Metadata["request_id"],
	)

	return nil
}

// jsonError encodes a simple {"error": msg} JSON payload.
func jsonError(msg string) []byte {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return b
}

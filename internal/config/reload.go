package config

import (
	"fmt"
	"net/url"
	"strings"

	"hydragate/internal/app"
	"hydragate/internal/plugin"
	"hydragate/internal/proxy"
)

// ValidateConfig validates the gateway configuration.
func ValidateConfig(cfg *app.GatewayConfig) error {
	if cfg.JWTSecret == "" {
		return fmt.Errorf("jwt_secret cannot be empty")
	}

	if len(cfg.APIKeys) == 0 {
		return fmt.Errorf("at least one api_key must be defined")
	}

	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.Capacity <= 0 {
			return fmt.Errorf("rate_limit.capacity must be positive when enabled")
		}
		if cfg.RateLimit.RefillRate <= 0 {
			return fmt.Errorf("rate_limit.refill_rate must be positive when enabled")
		}
	}

	if len(cfg.Routes) == 0 {
		return fmt.Errorf("at least one route must be defined")
	}

	for _, route := range cfg.Routes {
		if route.Route == "" {
			return fmt.Errorf("route prefix cannot be empty")
		}
		if route.Target == "" {
			return fmt.Errorf("target cannot be empty for route: %s", route.Route)
		}

		targetURL, err := url.Parse(route.Target)
		if err != nil {
			return fmt.Errorf("invalid target URL for route %s: %w", route.Route, err)
		}

		if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
			return fmt.Errorf("target scheme must be http or https for route: %s", route.Route)
		}

		if route.Transform.PathRewrite != "" {
			if !strings.HasPrefix(route.Transform.PathRewrite, "/") && !strings.HasPrefix(route.Transform.PathRewrite, "*") {
				return fmt.Errorf("path_rewrite must start with / or * for route: %s", route.Route)
			}
		}
	}

	return nil
}

// Reload loads a fresh config from disk, validates it, rebuilds the route
// registry and plugin executor, then atomically swaps all three into State.
// The pluginRegistry is the already-initialised PluginRegistry that has all
// built-in (and external) factories registered — it is reused across reloads
// so external .so plugins do not need to be re-opened.
func Reload(state *State, configPath string, pluginRegistry *plugin.PluginRegistry) error {
	newCfg, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := ValidateConfig(newCfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	newReg := proxy.NewRegistry()
	newReg.LoadRoutes(newCfg.Routes)

	newExec := plugin.NewExecutor(pluginRegistry)
	if err := newExec.UpdateConfig(newCfg.Plugins); err != nil {
		return fmt.Errorf("failed to update plugin config: %w", err)
	}

	// Swap all three atomically — config and registry first so that any
	// concurrent request reading the executor sees a consistent view.
	state.SetConfig(newCfg)
	state.SetRegistry(newReg)
	state.SetExecutor(newExec)

	return nil
}

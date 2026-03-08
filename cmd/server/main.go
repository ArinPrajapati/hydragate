package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"hydragate/internal/app"
	"hydragate/internal/cache"
	"hydragate/internal/config"
	"hydragate/internal/plugin"
	"hydragate/internal/plugins"
	"hydragate/internal/proxy"

	"github.com/redis/go-redis/v9"
)

// rdb is package-level so buildMainHandler and reload can share the same client.
var rdb *redis.Client

// mainHandler is swapped atomically on hot-reload.
var mainHandler http.Handler

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func handlerReload(state *config.State, pluginRegistry *plugin.PluginRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}

		if err := config.Reload(state, "config.json", pluginRegistry); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			slog.Error("manual reload failed", "error", err)
			return
		}

		// Rebuild the main handler so the new executor and cache config are live.
		mainHandler = buildMainHandler(rdb, state)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "reloaded"})
		slog.Info("config manually reloaded")
	}
}

// routePrefix extracts the first path segment (e.g. "api" from "/api/users").
func routePrefix(r *http.Request) string {
	trimmed := strings.TrimPrefix(r.URL.Path, "/")
	prefix, _, _ := strings.Cut(trimmed, "/")
	return prefix
}

// registerBuiltinPlugins registers all built-in plugin factories into the
// provided registry. External plugins are loaded later via UpdateConfig when
// the config's external_paths list is processed.
func registerBuiltinPlugins(reg *plugin.PluginRegistry, rdbClient *redis.Client, state *config.State) {
	// Logger — runs OnPreRoute + OnPreResponse
	reg.Register("logger", plugins.LoggerFactory)

	// Rate limiter — runs OnPreRoute with Redis token bucket
	reg.Register("rate_limiter", plugins.NewRateLimiterFactory(rdbClient))

	// API key auth — runs OnPreUpstream; reads live keys from state on every request
	reg.Register("api_key_auth", plugins.NewAPIKeyAuthFactory(
		func() map[string]string {
			return state.GetConfig().APIKeys
		},
		func() map[string]bool {
			return state.GetRegistry().ProtectedRoutes()
		},
	))

	// JWT auth — runs OnPreUpstream; reads live secret/claims from state on every request
	reg.Register("jwt_auth", plugins.NewJWTAuthFactory(
		func() string {
			return state.GetConfig().JWTSecret
		},
		func() map[string]string {
			return state.GetConfig().ForwardClaims
		},
		func() map[string]bool {
			return state.GetRegistry().ProtectedRoutes()
		},
	))
}

func buildMainHandler(rdbClient *redis.Client, state *config.State) http.Handler {
	cfg := state.GetConfig()
	reg := state.GetRegistry()
	exec := state.GetExecutor()

	// Adapter: bridge proxy.Registry into the app.RouteConfig shape that
	// cache.Cache expects.
	getRouteForCache := func(prefix string) (app.RouteConfig, bool) {
		route, found := reg.GetRoute(prefix)
		if !found {
			return app.RouteConfig{}, false
		}
		return app.RouteConfig{
			Route:      prefix,
			Target:     route.Target,
			Protected:  route.Protected,
			Transform:  route.Transform,
			Cache:      route.Cache,
			CachePaths: route.CachePaths,
		}, true
	}

	// Cache middleware (standalone — sits between the plugin executor and the proxy).
	cacheMiddleware := cache.Cache(rdbClient, cfg, getRouteForCache)

	// Plugin executor middleware drives all 4 phases.
	pluginMiddleware := exec.Middleware(routePrefix)

	// Chain:  pluginMiddleware wraps (cacheMiddleware wraps proxy.Forward)
	//
	// Request flow:
	//   pluginMiddleware (PreRoute → PreUpstream)
	//     → cacheMiddleware (cache hit? serve; miss? continue)
	//       → proxy.Forward (backend call)
	//     ← cacheMiddleware (store on miss)
	//   ← pluginMiddleware (PostUpstream → PreResponse → Flush)
	return pluginMiddleware(
		cacheMiddleware(
			http.HandlerFunc(proxy.Forward(reg)),
		),
	)
}

func main() {
	redisAddr := "localhost:6379"
	configPath := "config.json"

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		log.Fatalf("invalid initial config: %v", err)
	}

	rdb = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	slog.Info("connected to redis")

	reg := proxy.NewRegistry()
	reg.LoadRoutes(cfg.Routes)

	// Bootstrap state with a temporary executor so registerBuiltinPlugins can
	// close over state.GetConfig() / state.GetRegistry() safely.
	tempExec := plugin.NewExecutor(plugin.NewRegistry())
	state := config.NewState(cfg, reg, tempExec)

	// Build the plugin registry and register all built-in factories.
	pluginRegistry := plugin.NewRegistry()
	registerBuiltinPlugins(pluginRegistry, rdb, state)

	// Build the real executor from the config's plugin block.
	exec := plugin.NewExecutor(pluginRegistry)
	if err := exec.UpdateConfig(cfg.Plugins); err != nil {
		log.Fatalf("failed to initialise plugin executor: %v", err)
	}
	state.SetExecutor(exec)

	mainHandler = buildMainHandler(rdb, state)

	http.HandleFunc("/health", handlerHealth)
	http.HandleFunc("/reload", handlerReload(state, pluginRegistry))
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler.ServeHTTP(w, r)
	}))

	go config.WatchConfig(configPath, state, pluginRegistry)

	slog.Info("server started", "addr", "http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"hydragate/internal/app"
	"hydragate/internal/cache"
	"hydragate/internal/config"
	"hydragate/internal/middleware"
	"hydragate/internal/proxy"

	"github.com/redis/go-redis/v9"
)

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func handlerReload(state *config.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}

		if err := config.Reload(state, "config.json"); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			slog.Error("manual reload failed", "error", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "reloaded"})
		slog.Info("config manually reloaded")
	}
}

func buildMainHandler(rdb *redis.Client, state *config.State) http.Handler {
	cfg := state.GetConfig()
	reg := state.GetRegistry()

	jwtAuth := middleware.JWTAuth(middleware.JWTAuthConfig{
		Secret:          cfg.JWTSecret,
		ForwardClaims:   cfg.ForwardClaims,
		ProtectedRoutes: reg.ProtectedRoutes(),
	})

	apiKeyAuth := middleware.APIKeyAuth(middleware.APIKeyAuthConfig{
		Keys:            cfg.APIKeys,
		ProtectedRoutes: reg.ProtectedRoutes(),
	})

	rateLimiter := middleware.RateLimiter(rdb, cfg.RateLimit)

	// Create a function to get route config by prefix
	getRouteFunc := func(prefix string) (app.RouteConfig, bool) {
		route, found := reg.GetRoute(prefix)
		return app.RouteConfig{
			Route:      prefix,
			Target:     route.Target,
			Protected:  route.Protected,
			Transform:  route.Transform,
			Cache:      route.Cache,
			CachePaths: route.CachePaths,
		}, found
	}

	cacheMiddleware := cache.Cache(rdb, cfg, getRouteFunc)

	return middleware.Chain(
		http.HandlerFunc(proxy.Forward(reg)),
		middleware.Logger,
		rateLimiter,
		jwtAuth,
		apiKeyAuth,
		cacheMiddleware,
	)
}

var mainHandler http.Handler

func main() {

	reddisAddr := "localhost:6379"
	config_path := "config.json"

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.LoadConfig(config_path)
	if err != nil {
		log.Fatal(err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		log.Fatalf("invalid initial config: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: reddisAddr,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	slog.Info("connected to redis")

	reg := proxy.NewRegistry()
	reg.LoadRoutes(cfg.Routes)

	state := config.NewState(cfg, reg)
	mainHandler = buildMainHandler(rdb, state)

	http.HandleFunc("/health", handlerHealth)
	http.HandleFunc("/reload", handlerReload(state))
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler.ServeHTTP(w, r)
	}))

	go config.WatchConfig("config.json", state)

	slog.Info("server started", "addr", "http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

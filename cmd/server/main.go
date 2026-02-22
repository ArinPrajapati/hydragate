package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"hydragate/internal/config"
	"hydragate/internal/middleware"
	"hydragate/internal/proxy"

	"github.com/redis/go-redis/v9"
)

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	slog.Info("connected to redis")

	reg := proxy.NewRegistry()
	reg.LoadRoutes(cfg.Routes)

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

	http.Handle("/health", middleware.Chain(
		http.HandlerFunc(handlerHealth),
		middleware.Logger,
	))

	http.Handle("/", middleware.Chain(
		http.HandlerFunc(proxy.Forward(reg)),
		middleware.Logger,
		rateLimiter,
		jwtAuth,
		apiKeyAuth,
	))

	slog.Info("server started", "addr", "http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

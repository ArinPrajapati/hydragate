package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"hydragate/internal/config"
	"hydragate/internal/middleware"
	"hydragate/internal/proxy"
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

	reg := proxy.NewRegistry()
	reg.LoadRoutes(cfg.Routes)

	jwtAuth := middleware.JWTAuth(middleware.JWTAuthConfig{
		Secret:          cfg.JWTSecret,
		ForwardClaims:   cfg.ForwardClaims,
		ProtectedRoutes: reg.ProtectedRoutes(),
	})

	http.Handle("/health", middleware.Chain(
		http.HandlerFunc(handlerHealth),
		middleware.Logger,
	))

	http.Handle("/", middleware.Chain(
		http.HandlerFunc(proxy.Forward(reg)),
		middleware.Logger,
		jwtAuth,
	))

	slog.Info("server started", "addr", "http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

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

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello, %s!", r.URL.Path[1:])
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func main() {
	// Initialize structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}

	reg := proxy.NewRegistry()
	reg.LoadRoutes(cfg)

	http.Handle("/health", middleware.Chain(http.HandlerFunc(handlerHealth), middleware.Logger))
	http.Handle("/", middleware.Chain(http.HandlerFunc(proxy.Forward(reg)), middleware.Logger))
	slog.Info("server started", "addr", "http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

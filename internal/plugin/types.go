package plugin

import (
	"context"
	"net/http"
	"time"

	"log/slog"
)

const CurrentAPIVersion = 1

type PluginPhase string

const (
	PhasePreRoute     PluginPhase = "pre_route"
	PhasePreUpstream  PluginPhase = "pre_upstream"
	PhasePostUpstream PluginPhase = "post_upstream"
	PhasePreResponse  PluginPhase = "pre_response"
)

type PluginContext struct {
	Ctx       context.Context
	Phase     PluginPhase
	Request   *http.Request
	Response  *ResponseCapture
	Route     interface{}
	StartTime time.Time
	Metadata  map[string]interface{}
	Abort     bool
	AbortCode int
	AbortBody []byte
}

type Plugin interface {
	Name() string
	APIVersion() int
	Init(config map[string]interface{}, logger *slog.Logger) error
	ValidateConfig(config map[string]interface{}) error
	Shutdown() error
	OnPreRoute(ctx *PluginContext) error
	OnPreUpstream(ctx *PluginContext) error
	OnPostUpstream(ctx *PluginContext) error
	OnPreResponse(ctx *PluginContext) error
	Metrics() map[string]float64
}

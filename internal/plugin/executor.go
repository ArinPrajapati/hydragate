package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

type PluginEntry struct {
	Name      string
	Enabled   bool
	Priority  int
	TimeoutMs int
	OnError   string
}

type PluginsConfig struct {
	ExternalPaths []string                          `json:"external_paths"`
	Global        []PluginEntry                     `json:"global"`
	Routes        map[string][]PluginEntry          `json:"routes"`
	Config        map[string]map[string]interface{} `json:"config"`
}

type PluginExecutor struct {
	registry     *PluginRegistry
	globalChain  []PluginEntry
	routeChains  map[string][]PluginEntry
	configs      map[string]map[string]any
	chainTimeout time.Duration
	mu           sync.RWMutex
}

func NewExecutor(registry *PluginRegistry) *PluginExecutor {
	return &PluginExecutor{
		registry:    registry,
		routeChains: make(map[string][]PluginEntry),
		configs:     make(map[string]map[string]interface{}),
	}
}

func (e *PluginExecutor) UpdateConfig(cfg PluginsConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.registry.LoadExternal(cfg.ExternalPaths); err != nil {
		return err
	}

	e.globalChain = cfg.Global
	e.routeChains = cfg.Routes
	e.configs = cfg.Config

	return nil
}

func (e *PluginExecutor) executeWithTimeout(
	p Plugin,
	ctx *PluginContext,
	phase PluginPhase,
	timeout time.Duration,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx.Ctx, timeout)
	defer cancel()

	originalCtx := ctx.Ctx
	ctx.Ctx = timeoutCtx
	defer func() { ctx.Ctx = originalCtx }()

	done := make(chan error, 1)
	go func() {
		var err error
		switch phase {
		case PhasePreRoute:
			err = p.OnPreRoute(ctx)
		case PhasePreUpstream:
			err = p.OnPreUpstream(ctx)
		case PhasePostUpstream:
			err = p.OnPostUpstream(ctx)
		case PhasePreResponse:
			err = p.OnPreResponse(ctx)
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		return fmt.Errorf("plugin %s timed out after %v", p.Name(), timeout)
	}
}

func (e *PluginExecutor) Execute(ctx *PluginContext, routePrefix string, phase PluginPhase) error {
	e.mu.RLock()
	chain := e.buildChain(routePrefix, phase)
	e.mu.RUnlock()

	for _, entry := range chain {
		if !entry.Enabled {
			continue
		}

		plugin, err := e.registry.CreateInstance(entry.Name, e.configs[entry.Name])
		if err != nil {
			if entry.OnError == "continue" {
				slog.Error("plugin init failed", "plugin", entry.Name, "error", err)
				continue
			}
			return err
		}

		timeout := time.Duration(entry.TimeoutMs) * time.Millisecond
		if timeout == 0 {
			timeout = 5 * time.Second
		}

		err = e.executeWithTimeout(plugin, ctx, phase, timeout)
		if err != nil {
			if entry.OnError == "continue" {
				slog.Error("plugin execution failed",
					"plugin", entry.Name,
					"phase", phase,
					"error", err,
					"request_id", ctx.Metadata["request_id"],
				)
				continue
			}
			return err
		}

		if ctx.Abort {
			return nil
		}
	}
	return nil
}

func (e *PluginExecutor) buildChain(routePrefix string, phase PluginPhase) []PluginEntry {
	var chain []PluginEntry

	chain = append(chain, e.globalChain...)

	if routePlugins, ok := e.routeChains[routePrefix]; ok {
		chain = append(chain, routePlugins...)
	}

	sort.SliceStable(chain, func(i, j int) bool {
		return chain[i].Priority < chain[j].Priority
	})

	if phase == PhasePostUpstream || phase == PhasePreResponse {
		for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
			chain[i], chain[j] = chain[j], chain[i]
		}
	}

	return chain
}

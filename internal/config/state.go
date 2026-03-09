package config

import (
	"sync"
	"sync/atomic"

	"hydragate/internal/app"
	"hydragate/internal/plugin"
	"hydragate/internal/proxy"
)

type State struct {
	config   atomic.Value
	registry atomic.Value
	executor atomic.Value
	mu       sync.RWMutex
}

// NewState creates a new state instance with the given configuration, registry, and executor.
func NewState(cfg *app.GatewayConfig, reg *proxy.Registry, exec *plugin.PluginExecutor) *State {
	s := &State{}
	s.config.Store(cfg)
	s.registry.Store(reg)
	s.executor.Store(exec)
	return s
}

func (s *State) GetConfig() *app.GatewayConfig {
	v := s.config.Load()
	if v == nil {
		return nil
	}
	return v.(*app.GatewayConfig)
}

// GetRegistry returns the current routing registry.
func (s *State) GetRegistry() *proxy.Registry {
	v := s.registry.Load()
	if v == nil {
		return nil
	}
	return v.(*proxy.Registry)
}

// GetExecutor returns the current plugin executor.
func (s *State) GetExecutor() *plugin.PluginExecutor {
	v := s.executor.Load()
	if v == nil {
		return nil
	}
	return v.(*plugin.PluginExecutor)
}

// SetConfig updates the gateway configuration.
func (s *State) SetConfig(cfg *app.GatewayConfig) {
	s.config.Store(cfg)
}

// SetRegistry updates the routing registry.
func (s *State) SetRegistry(reg *proxy.Registry) {
	s.registry.Store(reg)
}

// SetExecutor updates the plugin executor.
func (s *State) SetExecutor(exec *plugin.PluginExecutor) {
	s.executor.Store(exec)
}

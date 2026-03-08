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

func (s *State) GetRegistry() *proxy.Registry {
	v := s.registry.Load()
	if v == nil {
		return nil
	}
	return v.(*proxy.Registry)
}

func (s *State) GetExecutor() *plugin.PluginExecutor {
	v := s.executor.Load()
	if v == nil {
		return nil
	}
	return v.(*plugin.PluginExecutor)
}

func (s *State) SetConfig(cfg *app.GatewayConfig) {
	s.config.Store(cfg)
}

func (s *State) SetRegistry(reg *proxy.Registry) {
	s.registry.Store(reg)
}

func (s *State) SetExecutor(exec *plugin.PluginExecutor) {
	s.executor.Store(exec)
}

package config

import (
	"sync"
	"sync/atomic"

	"hydragate/internal/app"
	"hydragate/internal/proxy"
)

type State struct {
	config   atomic.Value
	registry atomic.Value
	mu       sync.RWMutex
}

func NewState(cfg *app.GatewayConfig, reg *proxy.Registry) *State {
	s := &State{}
	s.config.Store(cfg)
	s.registry.Store(reg)
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

func (s *State) SetConfig(cfg *app.GatewayConfig) {
	s.config.Store(cfg)
}

func (s *State) SetRegistry(reg *proxy.Registry) {
	s.registry.Store(reg)
}

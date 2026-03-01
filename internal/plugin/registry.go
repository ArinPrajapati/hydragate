package plugin

import (
	"fmt"
	"log/slog"
	"plugin"
	"sync"
)

type PluginRegistry struct {
	factories map[string]PluginFactory
	mu        sync.RWMutex
}

func NewRegistry() *PluginRegistry {
	return &PluginRegistry{
		factories: make(map[string]PluginFactory),
	}
}

func (r *PluginRegistry) Register(name string, factory PluginFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("plugin already registered: %s", name)
	}
	r.factories[name] = factory
	return nil
}

func (r *PluginRegistry) GetFactory(name string) (PluginFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[name]
	return f, ok
}

func (r *PluginRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

func (r *PluginRegistry) CreateInstance(name string, config map[string]interface{}) (Plugin, error) {
	factory, ok := r.GetFactory(name)
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	logger := slog.Default().With("plugin", name)
	return factory(config, logger)
}

func (r *PluginRegistry) LoadExternal(paths []string) error {
	for _, path := range paths {
		plug, err := plugin.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open plugin %s: %w", path, err)
		}

		sym, err := plug.Lookup("Factory")
		if err != nil {
			return fmt.Errorf("plugin %s missing Factory symbol: %w", path, err)
		}

		factory, ok := sym.(*PluginFactory)
		if !ok {
			return fmt.Errorf("plugin %s Factory has wrong type", path)
		}

		tempPlugin, err := (*factory)(nil, nil)
		if err != nil {
			return fmt.Errorf("plugin %s factory failed: %w", path, err)
		}

		if tempPlugin.APIVersion() != CurrentAPIVersion {
			return fmt.Errorf("plugin %s has incompatible API version %d (expected %d)",
				path, tempPlugin.APIVersion(), CurrentAPIVersion)
		}

		r.Register(tempPlugin.Name(), *factory)
	}
	return nil
}

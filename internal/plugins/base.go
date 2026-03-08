package plugins

import (
	"log/slog"

	"hydragate/internal/plugin"
)

// BasePlugin provides no-op defaults for every method in the Plugin interface.
// Embed this in your plugin struct and only override the phases you need.
type BasePlugin struct {
	Logger *slog.Logger
	Config map[string]interface{}
}

func (p *BasePlugin) APIVersion() int { return plugin.CurrentAPIVersion }

func (p *BasePlugin) Init(cfg map[string]interface{}, logger *slog.Logger) error {
	p.Config = cfg
	p.Logger = logger
	return nil
}

func (p *BasePlugin) ValidateConfig(_ map[string]interface{}) error { return nil }

func (p *BasePlugin) Shutdown() error { return nil }

func (p *BasePlugin) OnPreRoute(_ *plugin.PluginContext) error { return nil }

func (p *BasePlugin) OnPreUpstream(_ *plugin.PluginContext) error { return nil }

func (p *BasePlugin) OnPostUpstream(_ *plugin.PluginContext) error { return nil }

func (p *BasePlugin) OnPreResponse(_ *plugin.PluginContext) error { return nil }

func (p *BasePlugin) Metrics() map[string]float64 { return nil }

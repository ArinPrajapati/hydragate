package plugin

import (
	"log/slog"
)

type PluginFactory func(config map[string]interface{}, logger *slog.Logger) (Plugin, error)

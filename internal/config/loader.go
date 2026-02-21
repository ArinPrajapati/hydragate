package config

import (
	"fmt"

	"hydragate/internal/proxy"
)

func LoadConfig(path string, reg *proxy.Registry) error {
	config, err := ParseConfig(path)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	for _, r := range config {
		reg.AddRoute(r.Route, r.Target)
	}

	return nil
}

package config

import (
	"fmt"

	"hydragate/internal/app"
)

// LoadConfig loads and parses the gateway configuration from a file.
func LoadConfig(path string) (*app.GatewayConfig, error) {
	config, err := ParseConfig(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

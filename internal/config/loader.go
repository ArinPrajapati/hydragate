package config

import (
	"fmt"

	"hydragate/internal/app"
)

func LoadConfig(path string) ([]app.RouteConfig, error) {
	config, err := ParseConfig(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

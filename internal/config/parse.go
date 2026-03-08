// Package config handles configuration loading, validation, and hot-reloading.
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"hydragate/internal/app"
)

// ParseConfig reads and parses the gateway configuration from a JSON file.
func ParseConfig(filePath string) (*app.GatewayConfig, error) {
	data, err := os.ReadFile(FilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config app.GatewayConfig

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling config file: %w", err)
	}

	return &config, nil
}

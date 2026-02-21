package config

import (
	"encoding/json"
	"fmt"
	"os"

	"hydragate/internal/app"
)

func ParseConfig(FilePath string) (*app.GatewayConfig, error) {
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

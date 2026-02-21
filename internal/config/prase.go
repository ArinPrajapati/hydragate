package config

import (
	"encoding/json"
	"fmt"
	"os"

	"hydragate/internal/app"
)

func ParseConfig(FilePath string) ([]app.RouteConfig, error) {
	data, err := os.ReadFile(FilePath)
	if err != nil {
		return nil, fmt.Errorf("Error reading config file:", err)
	}

	var routes []app.RouteConfig

	err = json.Unmarshal(data, &routes)
	if err != nil {
		return nil, fmt.Errorf("Error Unmarshalling config file:", err)
	}

	return routes, nil
}

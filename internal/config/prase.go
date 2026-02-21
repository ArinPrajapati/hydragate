package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Route  string `json:"route"`
	Target string `json:"target"`
}

func ParseConfig(FilePath string) ([]Config, error) {
	data, err := os.ReadFile(FilePath)
	if err != nil {
		return nil, fmt.Errorf("Error reading config file:", err)
	}

	var routes []Config

	err = json.Unmarshal(data, &routes)
	if err != nil {
		return nil, fmt.Errorf("Error Unmarshalling config file:", err)
	}

	return routes, nil
}

// Copyright 2026 The MathWorks, Inc.

package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ActionsConfig struct {
	Backend string               `json:"backend"`
	BaseURL string               `json:"baseURL"`
	RepoKey string               `json:"repoKey"`
	Auth    AuthConfig           `json:"auth"`
	Actions map[string]actionDef `json:"actions"`
}

type AuthConfig struct {
	Strategy string `json:"strategy"`
	TokenEnv string `json:"tokenEnv"`
}

type actionDef struct {
	Method           string          `json:"method"`
	URL              string          `json:"url"`
	ContentType      string          `json:"contentType"`
	Body             string          `json:"body"`
	ResponseStrategy string          `json:"responseStrategy"`
	ResponseMapping  responseMapping `json:"responseMapping"`
}

type responseMapping struct {
	Items   string `json:"items"`
	ItemURL string `json:"itemURL"`
}

// LoadActions reads and parses an actions JSON file.
// Config fields wrapped in {ENV_VAR_NAME} are resolved from environment variables.
func LoadActions(path string) (*ActionsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading actions file: %w", err)
	}
	var cfg ActionsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing actions file: %w", err)
	}

	if resolved, err := resolveEnvPlaceholder(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("resolving baseURL: %w", err)
	} else {
		cfg.BaseURL = resolved
	}
	if resolved, err := resolveEnvPlaceholder(cfg.RepoKey); err != nil {
		return nil, fmt.Errorf("resolving repoKey: %w", err)
	} else {
		cfg.RepoKey = resolved
	}

	return &cfg, nil
}

func resolveEnvPlaceholder(val string) (string, error) {
	if !strings.HasPrefix(val, "{") || !strings.HasSuffix(val, "}") {
		return val, nil
	}
	envName := val[1 : len(val)-1]
	envVal := os.Getenv(envName)
	if envVal == "" {
		return "", fmt.Errorf("env var %s is empty or not set", envName)
	}
	return envVal, nil
}

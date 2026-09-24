// Copyright 2026 The MathWorks, Inc.

package backend

import (
	"encoding/json"
	"fmt"
	"net/url"
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
// Trailing slashes are trimmed from baseURL and repoKey, and baseURL must be an
// absolute http or https URL.
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

	// URLs are built by joining baseURL and repoKey with "/" separators, so trim
	// operator-supplied slashes to avoid empty path segments such as
	// "https://host/artifactory//api/search/aql".
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	cfg.RepoKey = strings.Trim(cfg.RepoKey, "/")

	if err := validateBaseURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("invalid baseURL: %w", err)
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

// validateBaseURL rejects base URLs that would otherwise fail at request time
// with an opaque transport error, such as a host name with no scheme.
func validateBaseURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("must not be empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%q is not a valid URL: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%q must start with http:// or https://, for example https://artifactory.example.com/artifactory", raw)
	}
	// Hostname strips any port, so this also rejects a bare port such as
	// "http://:8080", which url.Parse reports as a non-empty Host.
	if u.Hostname() == "" {
		return fmt.Errorf("%q must include a host, for example https://artifactory.example.com/artifactory", raw)
	}
	return nil
}

// Copyright 2026 The MathWorks, Inc.

package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
)

type SourceLimits struct {
	Timeout             time.Duration
	MaxAQLResponseBytes int64
	MaxManifestBytes    int64
}

// Source fetches manifests from Artifactory using AQL via actions config.
// It implements index.ManifestSource.
type Source struct {
	actions       ActionsConfig
	client        *http.Client
	tokenOverride string
	limits        SourceLimits
}

func NewSource(actions ActionsConfig, limits SourceLimits) *Source {
	return &Source{
		actions: actions,
		client:  &http.Client{Timeout: limits.Timeout},
		limits:  limits,
	}
}

func NewSourceWithToken(actions ActionsConfig, limits SourceLimits, token string) *Source {
	return &Source{
		actions:       actions,
		client:        &http.Client{Timeout: limits.Timeout},
		tokenOverride: token,
		limits:        limits,
	}
}

func (s *Source) setAuth(req *http.Request) error {
	if s.tokenOverride != "" {
		req.Header.Set("Authorization", "Bearer "+s.tokenOverride)
		return nil
	}
	return applyAuth(req, s.actions.Auth)
}

func (s *Source) baseVars() map[string]string {
	return map[string]string{
		"baseURL": s.actions.BaseURL,
		"repoKey": s.actions.RepoKey,
	}
}

func (s *Source) ListManifests() ([]domain.PackageManifest, error) {
	action, ok := s.actions.Actions["list_manifests"]
	if !ok {
		return nil, fmt.Errorf("list_manifests action not defined")
	}

	urls, err := s.executeListAction(action)
	if err != nil {
		return nil, fmt.Errorf("listing manifests: %w", err)
	}

	manifests := make([]domain.PackageManifest, 0, len(urls))
	for _, u := range urls {
		m, err := s.fetchManifest(u)
		if err != nil {
			return nil, fmt.Errorf("fetching manifest from %s: %w", u, err)
		}
		manifests = append(manifests, *m)
	}

	return manifests, nil
}

func (s *Source) executeListAction(action actionDef) ([]string, error) {
	vars := s.baseVars()
	url := expandTemplate(action.URL, vars)
	body := expandTemplate(action.Body, vars)

	req, err := http.NewRequest(action.Method, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	if action.ContentType != "" {
		req.Header.Set("Content-Type", action.ContentType)
	}
	if err := s.setAuth(req); err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list_manifests returned status %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, s.limits.MaxAQLResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(respBody)) > s.limits.MaxAQLResponseBytes {
		return nil, fmt.Errorf("AQL response body exceeds limit of %d bytes", s.limits.MaxAQLResponseBytes)
	}

	switch action.ResponseStrategy {
	case "artifactory-aql":
		return parseAQLResponse(respBody, action.ResponseMapping, vars)
	default:
		return nil, fmt.Errorf("unsupported response strategy: %s", action.ResponseStrategy)
	}
}

func (s *Source) fetchManifest(url string) (*domain.PackageManifest, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if err := s.setAuth(req); err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET manifest returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, s.limits.MaxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > s.limits.MaxManifestBytes {
		return nil, fmt.Errorf("manifest response body exceeds limit of %d bytes", s.limits.MaxManifestBytes)
	}
	var m domain.PackageManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func applyAuth(req *http.Request, auth AuthConfig) error {
	switch auth.Strategy {
	case "bearer":
		token := os.Getenv(auth.TokenEnv)
		if token == "" {
			return fmt.Errorf("env var %s is empty or not set", auth.TokenEnv)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	case "", "none":
		// no auth
	default:
		return fmt.Errorf("unsupported auth strategy: %s", auth.Strategy)
	}
	return nil
}

func expandTemplate(tmpl string, vars map[string]string) string {
	result := tmpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

func parseAQLResponse(body []byte, mapping responseMapping, vars map[string]string) ([]string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing AQL response: %w", err)
	}

	itemsRaw, ok := raw[mapping.Items]
	if !ok {
		return nil, fmt.Errorf("key %q not found in AQL response", mapping.Items)
	}

	var items []map[string]any
	if err := json.Unmarshal(itemsRaw, &items); err != nil {
		return nil, fmt.Errorf("parsing items array: %w", err)
	}

	urls := make([]string, 0, len(items))
	for _, item := range items {
		itemVars := make(map[string]string, len(vars)+len(item))
		for k, v := range vars {
			itemVars[k] = v
		}
		for k, v := range item {
			itemVars["item."+k] = fmt.Sprintf("%v", v)
		}
		url := expandTemplate(mapping.ItemURL, itemVars)
		urls = append(urls, url)
	}

	return urls, nil
}

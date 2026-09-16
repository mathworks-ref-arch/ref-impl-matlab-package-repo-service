// Copyright 2026 The MathWorks, Inc.

package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
)

var (
	uuidRe   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	semverRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	nameRe   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
)

type PackageManifest struct {
	Version                   string          `json:"version"`
	Name                      string          `json:"name"`
	ID                        string          `json:"id"`
	DisplayName               string          `json:"displayName"`
	Summary                   string          `json:"summary"`
	Provider                  Provider        `json:"provider"`
	ReleaseCompatibility      string          `json:"releaseCompatibility"`
	Dependencies              Dependencies    `json:"dependencies"`
	SupportedPlatforms        json.RawMessage `json:"supportedPlatforms"`
	FormerNames               []string        `json:"formerNames"`
	Archives                  []Archive       `json:"archives"`
	RequiredAdditionalSoftware []string       `json:"requiredAdditionalSoftware"`
}

type Provider struct {
	Name         string `json:"name"`
	Organization string `json:"organization"`
}

type Dependencies []Dependency

func (d *Dependencies) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '[' {
		var arr []Dependency
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*d = arr
		return nil
	}
	var single Dependency
	if err := json.Unmarshal(data, &single); err == nil && single.Name != "" {
		*d = Dependencies{single}
		return nil
	}
	var obj map[string]Dependency
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	result := make(Dependencies, 0, len(obj))
	for name, dep := range obj {
		dep.Name = name
		result = append(result, dep)
	}
	*d = result
	return nil
}

type Dependency struct {
	Name               string `json:"name"`
	ID                 string `json:"id"`
	CompatibleVersions string `json:"compatibleVersions"`
}

type Archive struct {
	Platforms []string `json:"platforms"`
	URL       string   `json:"url"`
	Size      int64    `json:"size"`
	Digests   []Digest `json:"digests"`
}

type Digest struct {
	Alg    string `json:"alg"`
	Digest string `json:"digest"`
}

func (m *PackageManifest) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("%w: id is required", ErrValidation)
	}
	if !uuidRe.MatchString(m.ID) {
		return fmt.Errorf("%w: invalid id format: %s", ErrValidation, m.ID)
	}
	if m.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if !nameRe.MatchString(m.Name) {
		return fmt.Errorf("%w: invalid name format: %s", ErrValidation, m.Name)
	}
	if m.Version == "" {
		return fmt.Errorf("%w: version is required", ErrValidation)
	}
	if !semverRe.MatchString(m.Version) {
		return fmt.Errorf("%w: invalid semver: %s", ErrValidation, m.Version)
	}
	return nil
}

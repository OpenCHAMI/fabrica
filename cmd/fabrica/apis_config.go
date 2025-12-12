// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// APIsConfigFileName is the name of the file that stores API group and version
// declarations in the project root. This file is the single source of truth for
// API versioning, hub/spoke configuration, and external type imports.
const APIsConfigFileName = "apis.yaml"

// APIsConfig defines API groups and versions for hub/spoke generation.
// It is the root configuration structure stored in apis.yaml and drives all
// versioned code generation. Each project should have exactly one apis.yaml
// file in the project root.
type APIsConfig struct {
	Groups []APIGroup `yaml:"groups"`
}

// APIGroup describes a single API group and its version graph.
// It defines the fully qualified group name (e.g., "infra.example.io"),
// the storage/hub version used for persistence, all exposed API versions
// (including hub and spokes), the list of resource kinds, and any external
// type imports.
//
// Currently, only a single API group per project is supported. Multiple
// groups may be added in future versions.
type APIGroup struct {
	Name           string      `yaml:"name"`
	StorageVersion string      `yaml:"storageVersion"`
	Versions       []string    `yaml:"versions"`
	Resources      []string    `yaml:"resources,omitempty"`
	Imports        []APIImport `yaml:"imports,omitempty"`
}

// APIImport exposes external types for reuse in generated APIs.
// This allows projects to import Spec and Status types from other Go modules
// instead of defining them locally. Useful for shared type libraries or
// consuming types from dependency services.
//
// Example: importing DeviceSpec from a shared networking types package.
type APIImport struct {
	Module   string       `yaml:"module"`
	Tag      string       `yaml:"tag,omitempty"`
	Packages []APIPackage `yaml:"packages,omitempty"`
}

// APIPackage identifies a package within an imported module and the specific
// resource kinds to expose. The Path is relative to the module root.
type APIPackage struct {
	Path   string        `yaml:"path"`
	Expose []ExposedKind `yaml:"expose,omitempty"`
}

// ExposedKind maps remote Spec and Status types into the local API surface.
// The Kind field names the resource in the generated API, while SpecFrom and
// StatusFrom reference fully qualified type names from the imported package.
//
// Example:
//
//	kind: Device
//	specFrom: github.com/org/netmodel/api/types.DeviceSpec
//	statusFrom: github.com/org/netmodel/api/types.DeviceStatus
type ExposedKind struct {
	Kind       string `yaml:"kind"`
	SpecFrom   string `yaml:"specFrom,omitempty"`
	StatusFrom string `yaml:"statusFrom,omitempty"`
}

// LoadAPIsConfig reads and parses apis.yaml from the specified directory.
// If dir is empty, the current working directory is used.
//
// The function performs full validation of the loaded configuration, ensuring:
//   - At least one API group is defined
//   - All required fields (name, storageVersion, versions) are present
//   - The storageVersion appears in the versions list
//
// Returns an error if the file doesn't exist, cannot be parsed, or fails validation.
func LoadAPIsConfig(dir string) (*APIsConfig, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	cfgPath := filepath.Join(dir, APIsConfigFileName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", APIsConfigFileName, err)
	}

	var cfg APIsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", APIsConfigFileName, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveAPIsConfig writes apis.yaml to the specified directory.
// If dir is empty, the current working directory is used.
//
// The configuration is validated before writing. Returns an error if
// validation fails, the directory is inaccessible, or the write operation
// fails. The file is written with 0644 permissions.
func SaveAPIsConfig(dir string, cfg *APIsConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid apis.yaml: %w", err)
	}

	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal apis.yaml: %w", err)
	}

	cfgPath := filepath.Join(dir, APIsConfigFileName)
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", APIsConfigFileName, err)
	}

	return nil
}

// DefaultAPIsConfig builds a minimal valid apis.yaml configuration.
// This is used during project initialization to create a scaffold configuration
// with sensible defaults.
//
// Parameters:
//   - group: API group name (e.g., "infra.example.io"). Defaults to "example.fabrica.dev" if empty.
//   - storageVersion: Hub version for storage (e.g., "v1"). Defaults to "v1" if empty.
//   - versions: List of all versions to expose. Defaults to [storageVersion] if empty.
//
// The function ensures the storageVersion is always included in the versions list.
func DefaultAPIsConfig(group, storageVersion string, versions []string) *APIsConfig {
	resolvedGroup := group
	if resolvedGroup == "" {
		resolvedGroup = "example.fabrica.dev"
	}

	resolvedStorage := storageVersion
	if resolvedStorage == "" {
		resolvedStorage = "v1"
	}

	resolvedVersions := versions
	if len(resolvedVersions) == 0 {
		resolvedVersions = []string{resolvedStorage}
	}

	// Ensure storage version is listed.
	found := false
	for _, v := range resolvedVersions {
		if v == resolvedStorage {
			found = true
			break
		}
	}
	if !found {
		resolvedVersions = append([]string{resolvedStorage}, resolvedVersions...)
	}

	return &APIsConfig{
		Groups: []APIGroup{
			{
				Name:           resolvedGroup,
				StorageVersion: resolvedStorage,
				Versions:       resolvedVersions,
				Resources:      []string{},
			},
		},
	}
}

// Validate ensures the configuration is complete and consistent.
//
// Validation rules:
//   - At least one API group must be defined
//   - Each group must have a non-empty name
//   - Each group must specify a storageVersion
//   - Each group must list at least one version
//   - The storageVersion must appear in the versions list
//
// Returns a descriptive error if any validation rule is violated.
func (c *APIsConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("apis config is nil")
	}
	if len(c.Groups) == 0 {
		return fmt.Errorf("apis.yaml must define at least one group")
	}

	for _, g := range c.Groups {
		if g.Name == "" {
			return fmt.Errorf("apis.yaml group.name is required")
		}
		if g.StorageVersion == "" {
			return fmt.Errorf("apis.yaml group %s missing storageVersion", g.Name)
		}
		if len(g.Versions) == 0 {
			return fmt.Errorf("apis.yaml group %s must list at least one version", g.Name)
		}

		found := false
		for _, v := range g.Versions {
			if v == g.StorageVersion {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("storageVersion %s must appear in versions for group %s", g.StorageVersion, g.Name)
		}
	}

	return nil
}

// primaryGroup returns the first (and currently only supported) API group.
//
// Multiple API groups in a single project are planned for future versions,
// but not yet implemented. This function validates the configuration and
// returns an error if more than one group is defined.
//
// Returns the primary group or an error if validation fails or multiple
// groups are configured.
func (c *APIsConfig) primaryGroup() (*APIGroup, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if len(c.Groups) > 1 {
		return nil, fmt.Errorf("multiple API groups are not yet supported; configure a single group in apis.yaml")
	}
	return &c.Groups[0], nil
}

// addResource appends a resource name to the primary group's resource list
// if it isn't already present. This is called automatically by
// 'fabrica add resource' to maintain the resource inventory in apis.yaml.
//
// If the configuration is invalid or multiple groups exist, the operation
// silently fails (returns without error). This is a convenience method for
// CLI operations that should not block on configuration issues.
func (c *APIsConfig) addResource(name string) {
	group, err := c.primaryGroup()
	if err != nil {
		return
	}
	for _, existing := range group.Resources {
		if existing == name {
			return
		}
	}
	group.Resources = append(group.Resources, name)
}

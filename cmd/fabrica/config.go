// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = ".fabrica.yaml"

// FabricaConfig represents the complete configuration for a Fabrica project.
// This is stored in .fabrica.yaml in the project root.
type FabricaConfig struct {
	Project    ProjectConfig    `yaml:"project"`
	Features   FeaturesConfig   `yaml:"features"`
	Generation GenerationConfig `yaml:"generation"`
}

// ProjectConfig contains project metadata.
type ProjectConfig struct {
	Name        string    `yaml:"name"`
	Module      string    `yaml:"module"`
	Description string    `yaml:"description,omitempty"`
	Created     time.Time `yaml:"created"`
}

// FeaturesConfig defines which features are enabled for the project.
type FeaturesConfig struct {
	Validation     ValidationConfig     `yaml:"validation"`
	Events         EventsConfig         `yaml:"events"`
	Conditional    ConditionalConfig    `yaml:"conditional"`
	Versioning     VersioningConfig     `yaml:"versioning"`
	Auth           AuthConfig           `yaml:"auth"`
	Storage        StorageConfig        `yaml:"storage"`
	Metrics        MetricsConfig        `yaml:"metrics,omitempty"`
	Reconciliation ReconciliationConfig `yaml:"reconciliation,omitempty"`
}

// ValidationConfig controls validation behavior.
type ValidationConfig struct {
	Enabled bool   `yaml:"enabled"`
	Mode    string `yaml:"mode"` // strict, warn, disabled
}

// EventsConfig controls CloudEvents integration.
type EventsConfig struct {
	Enabled bool   `yaml:"enabled"`
	BusType string `yaml:"bus_type"` // memory, nats, kafka
}

// ConditionalConfig controls ETag and conditional request handling.
type ConditionalConfig struct {
	Enabled       bool   `yaml:"enabled"`
	ETagAlgorithm string `yaml:"etag_algorithm"` // sha256, md5
}

// VersioningConfig controls API versioning.
type VersioningConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Strategy       string `yaml:"strategy"`        // header, url, both
	DefaultVersion string `yaml:"default_version"` // v1, v2, etc.
}

// AuthConfig controls authorization/authentication.
type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider,omitempty"` // jwt, oauth2, custom
}

// StorageConfig controls storage backend.
type StorageConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"`                // file, ent
	DBDriver string `yaml:"db_driver,omitempty"` // postgres, mysql, sqlite, sqlite3
}

// MetricsConfig controls metrics/observability.
type MetricsConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider,omitempty"` // prometheus, datadog
}

// ReconciliationConfig controls reconciliation framework.
type ReconciliationConfig struct {
	Enabled      bool `yaml:"enabled"`
	WorkerCount  int  `yaml:"worker_count,omitempty"`  // Number of reconciler workers (default: 5)
	RequeueDelay int  `yaml:"requeue_delay,omitempty"` // Default requeue delay in minutes (default: 5)
}

// GenerationConfig controls what gets generated.
type GenerationConfig struct {
	Handlers       bool `yaml:"handlers"`
	Storage        bool `yaml:"storage"`
	Client         bool `yaml:"client"`
	OpenAPI        bool `yaml:"openapi"`
	Events         bool `yaml:"events"`
	Middleware     bool `yaml:"middleware"`
	Reconciliation bool `yaml:"reconciliation"`
}

// LoadConfig reads .fabrica.yaml from the specified directory.
// If dir is empty, uses current directory.
func LoadConfig(dir string) (*FabricaConfig, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	configPath := filepath.Join(dir, ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", ConfigFileName, err)
	}

	var config FabricaConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", ConfigFileName, err)
	}

	return &config, nil
}

// SaveConfig writes .fabrica.yaml to the specified directory.
func SaveConfig(targetDir string, config *FabricaConfig) error {
	// Validate before saving
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(targetDir, ConfigFileName)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", ConfigFileName, err)
	}

	return nil
}

// ValidateConfig validates all configuration fields.
func ValidateConfig(config *FabricaConfig) error {
	// Validate project fields
	if config.Project.Name == "" {
		return fmt.Errorf("project.name is required")
	}
	if config.Project.Module == "" {
		return fmt.Errorf("project.module is required")
	}

	// Validate validation mode
	validModes := map[string]bool{"strict": true, "warn": true, "disabled": true}
	if config.Features.Validation.Mode != "" && !validModes[config.Features.Validation.Mode] {
		return fmt.Errorf("invalid validation.mode: %s (must be 'strict', 'warn', or 'disabled')",
			config.Features.Validation.Mode)
	}
	// Sync enabled flag with mode
	if config.Features.Validation.Mode == "disabled" {
		config.Features.Validation.Enabled = false
	}

	// Validate event bus type
	if config.Features.Events.Enabled {
		validBusTypes := map[string]bool{"memory": true, "nats": true, "kafka": true}
		if !validBusTypes[config.Features.Events.BusType] {
			return fmt.Errorf("invalid events.bus_type: %s (must be 'memory', 'nats', or 'kafka')",
				config.Features.Events.BusType)
		}
	}

	// Validate ETag algorithm
	if config.Features.Conditional.Enabled {
		validAlgos := map[string]bool{"sha256": true, "md5": true}
		if config.Features.Conditional.ETagAlgorithm != "" && !validAlgos[config.Features.Conditional.ETagAlgorithm] {
			return fmt.Errorf("invalid conditional.etag_algorithm: %s (must be 'sha256' or 'md5')",
				config.Features.Conditional.ETagAlgorithm)
		}
	}

	// Validate versioning strategy
	if config.Features.Versioning.Enabled {
		validStrategies := map[string]bool{"header": true, "url": true, "both": true}
		if !validStrategies[config.Features.Versioning.Strategy] {
			return fmt.Errorf("invalid versioning.strategy: %s (must be 'header', 'url', or 'both')",
				config.Features.Versioning.Strategy)
		}
	}

	// Validate storage type
	if config.Features.Storage.Enabled {
		validTypes := map[string]bool{"file": true, "ent": true}
		if !validTypes[config.Features.Storage.Type] {
			return fmt.Errorf("invalid storage.type: %s (must be 'file' or 'ent')",
				config.Features.Storage.Type)
		}

		// Validate DB driver if using ent
		if config.Features.Storage.Type == "ent" && config.Features.Storage.DBDriver != "" {
			validDrivers := map[string]bool{"postgres": true, "mysql": true, "sqlite": true, "sqlite3": true}
			if !validDrivers[config.Features.Storage.DBDriver] {
				return fmt.Errorf("invalid storage.db_driver: %s (must be 'postgres', 'mysql', 'sqlite', or 'sqlite3')",
					config.Features.Storage.DBDriver)
			}
		}
	}

	return nil
}

// NewDefaultConfig creates a new config with sensible defaults.
func NewDefaultConfig(name, module string) *FabricaConfig {
	return &FabricaConfig{
		Project: ProjectConfig{
			Name:    name,
			Module:  module,
			Created: time.Now(),
		},
		Features: FeaturesConfig{
			Validation: ValidationConfig{
				Enabled: true,
				Mode:    "strict",
			},
			Events: EventsConfig{
				Enabled: false,
				BusType: "memory",
			},
			Conditional: ConditionalConfig{
				Enabled:       true,
				ETagAlgorithm: "sha256",
			},
			Versioning: VersioningConfig{
				Enabled:        true,
				Strategy:       "header",
				DefaultVersion: "v1",
			},
			Auth: AuthConfig{
				Enabled: false,
			},
			Storage: StorageConfig{
				Enabled: true,
				Type:    "file",
			},
			Metrics: MetricsConfig{
				Enabled: false,
			},
		},
		Generation: GenerationConfig{
			Handlers:   true,
			Storage:    true,
			Client:     true,
			OpenAPI:    true,
			Events:     false,
			Middleware: true,
		},
	}
}

// ===== API Versioning Configuration (apis.yaml) =====

// APIsConfig represents the apis.yaml configuration for hub/spoke versioning.
// This is a separate configuration file (apis.yaml) from .fabrica.yaml.
type APIsConfig struct {
	Groups []APIGroup `yaml:"groups"`
}

// APIGroup represents an API group with multiple versions.
type APIGroup struct {
	Name           string        `yaml:"name"`           // e.g., "infra.example.io"
	StorageVersion string        `yaml:"storageVersion"` // Hub version (e.g., "v1")
	Versions       []string      `yaml:"versions"`       // All versions (spokes)
	Resources      []APIResource `yaml:"resources"`      // Resources in this group
	Imports        []APIImport   `yaml:"imports"`        // External type imports
}

// APIResource represents a resource within an API group.
type APIResource struct {
	Kind     string                      `yaml:"kind"`     // e.g., "Device"
	Mappings map[string]APIFieldMappings `yaml:"mappings"` // Version-specific mappings
}

// APIFieldMappings defines field transformations for a version.
type APIFieldMappings struct {
	Renames []APIFieldRename `yaml:"renames"` // Field renames
}

// APIFieldRename defines a field rename between versions.
type APIFieldRename struct {
	From string `yaml:"from"` // Old field name
	To   string `yaml:"to"`   // New field name
}

// APIImport represents an external module/package import.
type APIImport struct {
	Module   string       `yaml:"module"`   // e.g., "github.com/yourorg/netmodel"
	Tag      string       `yaml:"tag"`      // e.g., "v0.9.3"
	Packages []APIPackage `yaml:"packages"` // Packages to import
}

// APIPackage represents a package within an imported module.
type APIPackage struct {
	Path   string          `yaml:"path"`   // e.g., "api/types"
	Expose []APITypeExpose `yaml:"expose"` // Types to expose
}

// APITypeExpose defines which types to import and where from.
type APITypeExpose struct {
	Kind       string `yaml:"kind"`       // e.g., "Device"
	SpecFrom   string `yaml:"specFrom"`   // e.g., "github.com/yourorg/netmodel/api/types.DeviceSpec"
	StatusFrom string `yaml:"statusFrom"` // e.g., "github.com/yourorg/netmodel/api/types.DeviceStatus"
}

// LoadAPIsConfig reads apis.yaml from the specified directory.
// If dir is empty, uses current directory.
func LoadAPIsConfig(dir string) (*APIsConfig, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	configPath := filepath.Join(dir, "apis.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read apis.yaml: %w", err)
	}

	var config APIsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse apis.yaml: %w", err)
	}

	// Validate
	if err := ValidateAPIsConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid apis.yaml: %w", err)
	}

	return &config, nil
}

// ValidateAPIsConfig validates the apis.yaml configuration.
func ValidateAPIsConfig(config *APIsConfig) error {
	if len(config.Groups) == 0 {
		return fmt.Errorf("at least one API group is required")
	}

	for i, group := range config.Groups {
		if group.Name == "" {
			return fmt.Errorf("group[%d]: name is required", i)
		}
		if group.StorageVersion == "" {
			return fmt.Errorf("group[%d] (%s): storageVersion is required", i, group.Name)
		}
		if len(group.Versions) == 0 {
			return fmt.Errorf("group[%d] (%s): at least one version is required", i, group.Name)
		}

		// Ensure storageVersion is in the versions list
		found := false
		for _, v := range group.Versions {
			if v == group.StorageVersion {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("group[%d] (%s): storageVersion '%s' must be in versions list",
				i, group.Name, group.StorageVersion)
		}
	}

	return nil
}

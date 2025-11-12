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

// VersioningConfig controls API versioning (hub/spoke model).
type VersioningConfig struct {
	Enabled        bool     `yaml:"enabled"`
	Group          string   `yaml:"group"`           // e.g., "infra.example.io"
	StorageVersion string   `yaml:"storage_version"` // Hub version (e.g., "v1")
	Versions       []string `yaml:"versions"`        // All versions including hub (e.g., ["v1alpha1", "v1beta1", "v1"])
	Resources      []string `yaml:"resources"`       // Resource kinds (e.g., ["Device", "Sensor"])
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
	// Validate versioning configuration
	if err := ValidateVersioning(&config.Features.Versioning); err != nil {
		return fmt.Errorf("invalid versioning configuration: %w", err)
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
				Enabled:        false, // User must enable and configure
				Group:          "",
				StorageVersion: "",
				Versions:       []string{},
				Resources:      []string{},
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

// ValidateVersioning validates the versioning configuration in .fabrica.yaml.
func ValidateVersioning(config *VersioningConfig) error {
	if !config.Enabled {
		return nil // Validation only applies when enabled
	}

	if config.Group == "" {
		return fmt.Errorf("versioning.group is required when versioning is enabled")
	}
	if config.StorageVersion == "" {
		return fmt.Errorf("versioning.storage_version is required when versioning is enabled")
	}
	if len(config.Versions) == 0 {
		return fmt.Errorf("versioning.versions must have at least one version")
	}
	// Resources can be empty at init time - will be populated when resources are added

	// Ensure storage_version is in versions list
	found := false
	for _, v := range config.Versions {
		if v == config.StorageVersion {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("versioning.storage_version '%s' must be in versions list", config.StorageVersion)
	}

	return nil
}

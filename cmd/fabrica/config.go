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

// ConfigFileName is the name of the Fabrica project configuration file.
// This file stores generator behavior settings, feature toggles, and metadata.
// It is created in the project root by 'fabrica init' and should be committed
// to version control.
const ConfigFileName = ".fabrica.yaml"

// FabricaConfig represents the complete configuration for a Fabrica project.
// This is stored in .fabrica.yaml in the project root and controls generator
// behavior, feature toggles, and code generation options.
//
// The configuration is split into three main sections:
//   - Project: metadata about the project (name, module path, creation time)
//   - Features: feature toggles for validation, events, auth, storage, etc.
//   - Generation: controls which artifacts to generate (handlers, client, OpenAPI, etc.)
//
// Note: API versioning configuration has moved to apis.yaml. The Versioning
// field in FeaturesConfig is deprecated and ignored by the generator.
type FabricaConfig struct {
	Project    ProjectConfig    `yaml:"project"`
	Features   FeaturesConfig   `yaml:"features"`
	Generation GenerationConfig `yaml:"generation"`
}

// ProjectConfig contains project metadata.
// This information is primarily for documentation and tooling purposes.
type ProjectConfig struct {
	Name        string    `yaml:"name"`
	Module      string    `yaml:"module"`
	Description string    `yaml:"description,omitempty"`
	Created     time.Time `yaml:"created"`
}

// FeaturesConfig defines which features are enabled for the project.
// Each nested config structure controls a specific aspect of code generation
// and runtime behavior. Features can be toggled independently.
type FeaturesConfig struct {
	Validation     ValidationConfig     `yaml:"validation"`
	Events         EventsConfig         `yaml:"events"`
	Conditional    ConditionalConfig    `yaml:"conditional"`
	Auth           AuthConfig           `yaml:"auth"`
	Storage        StorageConfig        `yaml:"storage"`
	Metrics        MetricsConfig        `yaml:"metrics,omitempty"`
	Reconciliation ReconciliationConfig `yaml:"reconciliation,omitempty"`

	// Security controls TokenSmith-first AuthN/AuthZ generation.
	Security SecurityConfig `yaml:"security,omitempty"`
}

// ValidationConfig controls validation behavior.
// Determines whether struct field validation is enabled and how validation
// failures are handled (strict errors, warnings, or disabled).
type ValidationConfig struct {
	Enabled bool   `yaml:"enabled"`
	Mode    string `yaml:"mode"` // strict, warn, disabled
}

// EventsConfig controls CloudEvents integration.
// When enabled, the generator emits CloudEvents for resource lifecycle
// operations (create, update, delete) and condition changes. Supported
// bus types include memory (in-process), nats, and kafka.
type EventsConfig struct {
	Enabled bool   `yaml:"enabled"`
	BusType string `yaml:"bus_type"` // memory, nats, kafka
}

// ConditionalConfig controls ETag and conditional request handling.
// When enabled, generates middleware for If-Match, If-None-Match,
// If-Modified-Since, and If-Unmodified-Since headers. Supports sha256
// and md5 ETag algorithms.
type ConditionalConfig struct {
	Enabled       bool   `yaml:"enabled"`
	ETagAlgorithm string `yaml:"etag_algorithm"` // sha256, md5
}

// VersioningConfig controls API versioning (hub/spoke model).
//
// Deprecated: API versioning configuration has moved to apis.yaml in the
// project root. This structure is preserved for backward compatibility with
// existing projects but is no longer read by code generators. New projects
// should configure versioning in apis.yaml instead.
//
// See docs/apis-yaml.md for the current versioning configuration format.
type VersioningConfig struct {
	Enabled        bool     `yaml:"enabled,omitempty"`
	Group          string   `yaml:"group,omitempty"`
	StorageVersion string   `yaml:"storage_version,omitempty"`
	Versions       []string `yaml:"versions,omitempty"`
	Resources      []string `yaml:"resources,omitempty"`
}

// AuthConfig controls authorization/authentication.
// When enabled, generates middleware and handlers for authentication.
// Supported providers include jwt, oauth2, and custom implementations.
type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider,omitempty"` // jwt, oauth2, custom
}

// StorageConfig controls storage backend configuration.
// Determines the persistence layer for resources. Type can be 'file'
// (file-based JSON storage) or 'ent' (database with Ent ORM).
// When using 'ent', DBDriver specifies the database (postgres, mysql, sqlite).
type StorageConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"`                // file, ent
	DBDriver string `yaml:"db_driver,omitempty"` // postgres, mysql, sqlite, sqlite3
}

// MetricsConfig controls metrics/observability.
// When enabled, generates Prometheus metrics endpoints and instrumentation
// for monitoring resource operations, request latency, and error rates.
type MetricsConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider,omitempty"` // prometheus, datadog
}

// ReconciliationConfig controls reconciliation framework.
// When enabled, generates a Kubernetes-style controller-runtime reconciliation
// loop for event-driven resource management. WorkerCount controls parallelism,
// and RequeueDelay sets the default backoff for failed reconciliations.
type ReconciliationConfig struct {
	Enabled      bool `yaml:"enabled"`
	WorkerCount  int  `yaml:"worker_count,omitempty"`  // Number of reconciler workers (default: 5)
	RequeueDelay int  `yaml:"requeue_delay,omitempty"` // Default requeue delay in minutes (default: 5)
}

// GenerationConfig controls what gets generated.
// Each boolean field toggles generation of a specific artifact type.
// This allows incremental regeneration (e.g., regenerate only handlers
// without touching storage code).
type GenerationConfig struct {
	Handlers       bool `yaml:"handlers"`
	Storage        bool `yaml:"storage"`
	Client         bool `yaml:"client"`
	OpenAPI        bool `yaml:"openapi"`
	Events         bool `yaml:"events"`
	Middleware     bool `yaml:"middleware"`
	Reconciliation bool `yaml:"reconciliation"`
}

// LoadConfig reads and parses .fabrica.yaml from the specified directory.
// If dir is empty, the current working directory is used.
//
// Returns an error if the file doesn't exist or cannot be parsed.
// Does not perform validation; use ValidateConfig separately if needed.
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
// The configuration is validated before writing to ensure consistency.
// Returns an error if validation fails or the write operation fails.
// The file is written with 0644 permissions.
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

// ValidateConfig validates all configuration fields for correctness and consistency.
//
// Validation rules include:
//   - Project name and module path are required
//   - Validation mode must be 'strict', 'warn', or 'disabled'
//   - Event bus type must be 'memory', 'nats', or 'kafka'
//   - ETag algorithm must be 'sha256' or 'md5'
//   - Storage type must be 'file' or 'ent'
//   - DB driver (when using ent) must be 'postgres', 'mysql', 'sqlite', or 'sqlite3'
//
// Returns a descriptive error if any validation rule is violated.
func ValidateConfig(config *FabricaConfig) error {
	// Normalize security settings first (may emit warnings).
	config.Features.Security.AuthZ.Mode = normalizeSecurityMode(config.Features.Security.AuthZ.Mode, os.Stderr)

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

	// Deprecated: versioning config is now driven by apis.yaml and ignored here.

	// Security semantics:
	// 1) AuthZ requires AuthN.
	if config.Features.Security.AuthZ.Enabled && !config.Features.Security.AuthN.Enabled {
		return fmt.Errorf("features.security.authz.enabled requires features.security.authn.enabled")
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

// NewDefaultConfig creates a new configuration with sensible defaults.
// This is used during project initialization to create a scaffold .fabrica.yaml.
//
// Default settings:
//   - Validation: enabled in strict mode
//   - Events: disabled
//   - Conditional requests: enabled with SHA256 ETags
//   - Auth: disabled
//   - Storage: file-based (no database)
//   - Metrics: disabled
//   - Generation: all enabled except events and reconciliation
//
// Parameters:
//   - name: project name
//   - module: Go module path (e.g., github.com/user/project)
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
			Auth: AuthConfig{
				Enabled: false,
			},
			Security: SecurityConfig{
				AuthN: AuthNConfig{Enabled: false},
				AuthZ: AuthZConfig{Enabled: false, Mode: SecurityModeEnforce},
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
//
// Deprecated: This function is preserved for backward compatibility but is no
// longer used by the generator. API versioning validation now happens via
// APIsConfig.Validate() when loading apis.yaml.
//
// Validation rules (for legacy configs):
//   - If enabled, group name is required
//   - If enabled, storageVersion is required
//   - If enabled, at least one version must be listed
//   - The storageVersion must appear in the versions list
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

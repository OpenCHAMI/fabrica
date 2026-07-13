// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package config provides shared Fabrica project configuration models and helpers.
package config

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"

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
	Name           string       `yaml:"name"`
	StorageVersion string       `yaml:"storageVersion"`
	Versions       []string     `yaml:"versions"`
	Resources      APIResources `yaml:"resources,omitempty"`
	Imports        []APIImport  `yaml:"imports,omitempty"`
}

// APIResource controls generated output for a resource listed in an API group.
type APIResource struct {
	Name       string   `yaml:"-"`
	Path       string   `yaml:"path,omitempty"`
	Operations []string `yaml:"operations,omitempty"`
}

// Configured reports whether this resource has explicit generation settings.
func (r APIResource) Configured() bool {
	return r.Path != "" || len(r.Operations) > 0
}

// APIResources is the apis.yaml resource inventory. It accepts both the
// historical list form and the configured map form:
//
//	resources:
//	  - Device
//
//	resources:
//	  Device:
//	    path: /devices
//	    operations: [list, get]
type APIResources []APIResource

// Names returns the configured resource names in declaration order.
func (r APIResources) Names() []string {
	names := make([]string, 0, len(r))
	for _, resource := range r {
		names = append(names, resource.Name)
	}

	return names
}

// Get returns a resource by name.
func (r APIResources) Get(name string) (APIResource, bool) {
	for _, resource := range r {
		if resource.Name == name {
			return resource, true
		}
	}

	return APIResource{}, false
}

// Contains reports whether a resource name is present.
func (r APIResources) Contains(name string) bool {
	_, ok := r.Get(name)
	return ok
}

// UnmarshalYAML supports both list and map syntax for resources.
func (r *APIResources) UnmarshalYAML(value *yaml.Node) error {
	var resources APIResources
	switch value.Kind {
	// List syntax: resources: [Device, Network]
	case yaml.SequenceNode:
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("resources list entries must be resource name strings; use map syntax for resource configuration")
			}
			resources = append(resources, APIResource{Name: item.Value})
		}
	// Map syntax: resources: {Device: {path: /devices, operations: [list, get]}}
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			key := value.Content[i]
			val := value.Content[i+1]
			if err := validateResourceSettingsNode(key.Value, val); err != nil {
				return err
			}
			var resource APIResource
			if val.Kind != yaml.ScalarNode || val.Value != "" {
				if err := val.Decode(&resource); err != nil {
					return err
				}
			}
			resource.Name = key.Value
			resources = append(resources, resource)
		}
	default:
		return fmt.Errorf("resources must be a list or map")
	}
	*r = resources

	return nil
}

// validateResourceSettingsNode rejects unknown resource settings and
// distinguishes an omitted operations field, which selects Fabrica's defaults,
// from an explicitly empty operations list.
func validateResourceSettingsNode(resourceName string, value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(value.Content); i += 2 {
		field := value.Content[i].Value
		switch field {
		case "path":
			continue
		case "operations":
			var operations []string
			if err := value.Content[i+1].Decode(&operations); err != nil {
				return err
			}
			if len(operations) == 0 {
				return fmt.Errorf("resources.%s.operations must contain at least one operation", resourceName)
			}
			continue
		}
		return fmt.Errorf("resources.%s contains unsupported field %q", resourceName, field)
	}

	return nil
}

// MarshalYAML writes compact list syntax when no resources have explicit
// configuration, and map syntax once path/operation settings are present.
func (r APIResources) MarshalYAML() (interface{}, error) {
	hasConfig := false
	for _, resource := range r {
		if resource.Configured() {
			hasConfig = true
			break
		}
	}
	if !hasConfig {
		return r.Names(), nil
	}

	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, resource := range r {
		key := &yaml.Node{Kind: yaml.ScalarNode, Value: resource.Name}
		value := &yaml.Node{Kind: yaml.MappingNode}
		if resource.Path != "" {
			value.Content = append(value.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "path"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: resource.Path},
			)
		}
		if len(resource.Operations) > 0 {
			operations := &yaml.Node{Kind: yaml.SequenceNode}
			for _, operation := range resource.Operations {
				operations.Content = append(operations.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: operation})
			}
			value.Content = append(value.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "operations"},
				operations,
			)
		}
		node.Content = append(node.Content, key, value)
	}

	return node, nil
}

var validResourceOperations = map[string]struct{}{
	"list":          {},
	"get":           {},
	"create":        {},
	"update":        {},
	"put":           {},
	"patch":         {},
	"delete":        {},
	"updatestatus":  {},
	"update-status": {},
	"status-update": {},
	"put-status":    {},
	"patchstatus":   {},
	"patch-status":  {},
	"status-patch":  {},
	"read":          {},
	"write":         {},
	"status":        {},
	"all":           {},
	"crud":          {},
}

const (
	builtInHealthPath  = "/health"
	builtInOpenAPIPath = "/openapi.json"
	builtInDocsPath    = "/docs"
)

var (
	reservedResourcePaths = map[string]struct{}{
		builtInHealthPath:  {},
		builtInOpenAPIPath: {},
		builtInDocsPath:    {},
	}
	resourcePathSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
)

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
				Resources:      APIResources{},
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

		resources := make(map[string]struct{}, len(g.Resources))
		paths := make(map[string]string, len(g.Resources))
		for _, resource := range g.Resources {
			name := strings.TrimSpace(resource.Name)
			if name == "" {
				return fmt.Errorf("apis.yaml group %s contains a resource with an empty name", g.Name)
			}
			if _, ok := resources[name]; ok {
				return fmt.Errorf("apis.yaml group %s lists resource %s more than once", g.Name, name)
			}
			resources[name] = struct{}{}
			path := resource.Path
			if path == "" {
				path = "/" + strings.ToLower(name) + "s"
			}
			normalizedPath, err := validateResourcePath(name, path)
			if err != nil {
				return err
			}
			if existing, ok := paths[normalizedPath]; ok {
				return fmt.Errorf("resources.%s.path duplicates path %s already used by resource %s", name, normalizedPath, existing)
			}
			for existingPath, existingName := range paths {
				if resourcePathsOverlap(existingPath, normalizedPath) {
					return fmt.Errorf("resources.%s.path %s conflicts with generated routes for resource %s at %s", name, normalizedPath, existingName, existingPath)
				}
			}
			paths[normalizedPath] = name
			for _, operation := range resource.Operations {
				if _, ok := validResourceOperations[strings.ToLower(strings.TrimSpace(operation))]; !ok {
					return fmt.Errorf("resources.%s.operations contains unsupported operation %q", name, operation)
				}
			}
		}
	}

	return nil
}

func validateResourcePath(resourceName, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("resources.%s.path must not be empty", resourceName)
	}
	if path != strings.TrimSpace(path) {
		return "", fmt.Errorf("resources.%s.path must not contain surrounding whitespace", resourceName)
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("resources.%s.path must start with /", resourceName)
	}
	if path == "/" {
		return "", fmt.Errorf("resources.%s.path must not be /", resourceName)
	}
	if strings.HasSuffix(path, "/") {
		return "", fmt.Errorf("resources.%s.path must not end with /", resourceName)
	}
	if strings.Contains(path, "//") {
		return "", fmt.Errorf("resources.%s.path must not contain empty path segments", resourceName)
	}
	if pathpkg.Clean(path) != path {
		return "", fmt.Errorf("resources.%s.path must not contain . or .. segments", resourceName)
	}
	for _, segment := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if !resourcePathSegmentPattern.MatchString(segment) {
			return "", fmt.Errorf("resources.%s.path segment %q contains unsupported characters", resourceName, segment)
		}
	}
	if _, ok := reservedResourcePaths[path]; ok {
		return "", fmt.Errorf("resources.%s.path conflicts with built-in endpoint %s", resourceName, path)
	}
	if strings.HasPrefix(path, builtInDocsPath+"/") {
		return "", fmt.Errorf("resources.%s.path conflicts with built-in endpoint %s", resourceName, builtInDocsPath)
	}
	return path, nil
}

// resourcePathsOverlap reports whether either collection path can be consumed
// by the other resource's generated /{uid} or /{uid}/status routes.
func resourcePathsOverlap(first, second string) bool {
	return matchesGeneratedResourceRoute(first, second) || matchesGeneratedResourceRoute(second, first)
}

func matchesGeneratedResourceRoute(collectionPath, candidatePath string) bool {
	prefix := collectionPath + "/"
	if !strings.HasPrefix(candidatePath, prefix) {
		return false
	}

	segments := strings.Split(strings.TrimPrefix(candidatePath, prefix), "/")
	return len(segments) == 1 || (len(segments) == 2 && segments[1] == "status")
}

// PrimaryGroup returns the first (and currently only supported) API group.
//
// Multiple API groups in a single project are planned for future versions,
// but not yet implemented. This function validates the configuration and
// returns an error if more than one group is defined.
//
// Returns the primary group or an error if validation fails or multiple
// groups are configured.
func (c *APIsConfig) PrimaryGroup() (*APIGroup, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if len(c.Groups) > 1 {
		return nil, fmt.Errorf("multiple API groups are not yet supported; configure a single group in apis.yaml")
	}
	return &c.Groups[0], nil
}

// AddResource appends a resource name to the primary group's resource list
// if it isn't already present. This is called automatically by
// 'fabrica add resource' to maintain the resource inventory in apis.yaml.
//
// If the configuration is invalid or multiple groups exist, the operation
// silently fails (returns without error). This is a convenience method for
// CLI operations that should not block on configuration issues.
func (c *APIsConfig) AddResource(name string) {
	group, err := c.PrimaryGroup()
	if err != nil {
		return
	}
	if group.Resources.Contains(name) {
		return
	}
	group.Resources = append(group.Resources, APIResource{Name: name})
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const resourceVersioningConfiguration = "+fabrica:resource-versioning=enabled"

// ErrUnsupportedStorageFeature identifies incompatible generator feature and backend combinations.
var ErrUnsupportedStorageFeature = errors.New("unsupported storage feature")

// StorageFeatureError identifies a generator feature that its selected backend cannot implement.
type StorageFeatureError struct {
	ResourceName   string
	ConfigKey      string
	StorageBackend string
	SourcePath     string
}

func (e *StorageFeatureError) Error() string {
	location := e.SourcePath
	if location == "" {
		location = "generator configuration"
	}
	return fmt.Sprintf("%s: resource %s: configuration %q is unsupported by %s storage", location, e.ResourceName, e.ConfigKey, e.StorageBackend)
}

// Is classifies StorageFeatureError as ErrUnsupportedStorageFeature.
func (e *StorageFeatureError) Is(target error) bool {
	return target == ErrUnsupportedStorageFeature
}

// Resource returns the resource whose configuration is unsupported.
func (e *StorageFeatureError) Resource() string {
	return e.ResourceName
}

// Configuration returns the unsupported configuration directive.
func (e *StorageFeatureError) Configuration() string {
	return e.ConfigKey
}

// Backend returns the selected storage backend.
func (e *StorageFeatureError) Backend() string {
	return e.StorageBackend
}

func (g *Generator) validateVersioningStorage() error {
	if g.StorageType != "ent" {
		return nil
	}

	var errs []error
	for _, resource := range g.Resources {
		enabled, err := resourceVersioningEnabled(resource)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if enabled {
			errs = append(errs, &StorageFeatureError{
				ResourceName: resource.Name, ConfigKey: resourceVersioningConfiguration,
				StorageBackend: g.StorageType, SourcePath: resource.SourcePath,
			})
		}
	}
	return errors.Join(errs...)
}

func resourceVersioningEnabled(resource ResourceMetadata) (bool, error) {
	if resource.Tags["versioning"] == "enabled" {
		return true, nil
	}
	if resource.SourcePath == "" {
		return false, nil
	}

	content, err := os.ReadFile(resource.SourcePath)
	if err != nil {
		return false, fmt.Errorf("inspect resource versioning for %s from %s: %w", resource.Name, resource.SourcePath, err)
	}
	return strings.Contains(string(content), resourceVersioningConfiguration), nil
}

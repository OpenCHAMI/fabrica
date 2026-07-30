// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"

	"github.com/openchami/fabrica/pkg/annotations"
)

// RegisterResourceFromSource registers a resource and records its exact annotation source.
func (g *Generator) RegisterResourceFromSource(resourceType interface{}, sourcePath string) error {
	resourceIndex := len(g.Resources)
	if err := g.RegisterResource(resourceType); err != nil {
		return fmt.Errorf("register resource from %s: %w", sourcePath, err)
	}
	g.Resources[resourceIndex].SourcePath = sourcePath
	return nil
}

// PrepareResourceAnnotations resolves and validates every registered source before generation.
func (g *Generator) PrepareResourceAnnotations() error {
	if err := g.validateVersioningStorage(); err != nil {
		return err
	}

	resolved := make([]*annotations.ResourceAnnotations, len(g.Resources))
	errs := make([]error, 0)
	for resourceIndex := range g.Resources {
		resource := g.Resources[resourceIndex]
		if resource.SourcePath == "" {
			continue
		}

		resourceAnnotations, err := g.ParseResourceAnnotations(resource.SourcePath, resource.Name)
		if err != nil {
			errs = append(errs, fmt.Errorf("prepare annotations for %s from %s: %w", resource.Name, resource.SourcePath, err))
			continue
		}
		if err := annotations.ValidateStorageEnforcement(resourceAnnotations, annotations.StorageValidationContext{
			Filename: resource.SourcePath, ResourceName: resource.Name,
			Backend: annotations.StorageBackend(g.StorageType), Mode: resourceAnnotations.StorageMode,
		}); err != nil {
			errs = append(errs, fmt.Errorf("prepare annotations for %s from %s: %w", resource.Name, resource.SourcePath, err))
			continue
		}
		resolved[resourceIndex] = resourceAnnotations
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	for resourceIndex, resourceAnnotations := range resolved {
		if resourceAnnotations != nil {
			g.Resources[resourceIndex].Annotations = resourceAnnotations
		}
	}
	return nil
}

func (g *Generator) validateAnnotationsForDatabase(
	filePath string,
	resourceName string,
	resourceAnnotations *annotations.ResourceAnnotations,
) error {
	if g.DBDriver == "" {
		return nil
	}
	if resourceAnnotations.StorageMode != annotations.StorageModeDedicated {
		if err := annotations.ValidateForDatabase(resourceAnnotations, g.DBDriver); err != nil {
			return fmt.Errorf("annotations incompatible with %s: %w", g.DBDriver, err)
		}
		return nil
	}
	dialect, err := annotations.ParseDialect(g.DBDriver, annotations.SourcePosition{
		Filename: filePath,
		TypeName: resourceName,
	})
	if err != nil {
		return fmt.Errorf("parse annotation dialect for %s: %w", resourceName, err)
	}
	if _, err := annotations.ResolveStorageIntent(filePath, resourceName, dialect); err != nil {
		return fmt.Errorf("resolve storage annotations for %s: %w", resourceName, err)
	}
	return nil
}

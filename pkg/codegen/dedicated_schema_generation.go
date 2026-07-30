// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openchami/fabrica/pkg/annotations"
)

type preparedDedicatedSchema struct {
	resourceName string
	outputPath   string
	data         dedicatedSchemaData
}

type dedicatedSchemaPreparationError struct {
	resourceName string
	err          error
}

func (e *dedicatedSchemaPreparationError) Error() string {
	return fmt.Sprintf("prepare dedicated schema for %s: %v", e.resourceName, e.err)
}

func (e *dedicatedSchemaPreparationError) Unwrap() error {
	return e.err
}

func (g *Generator) prepareDedicatedSchemas(schemaDir string) ([]preparedDedicatedSchema, error) {
	prepared := make([]preparedDedicatedSchema, 0)
	for _, resource := range g.Resources {
		isDedicated := resource.Annotations != nil && resource.Annotations.StorageMode == annotations.StorageModeDedicated
		if !isDedicated {
			continue
		}

		outputPath := filepath.Join(schemaDir, strings.ToLower(resource.Name)+".go")
		data, err := g.dedicatedSchemaData(resource)
		if err != nil {
			return nil, &dedicatedSchemaPreparationError{resourceName: resource.Name, err: err}
		}
		prepared = append(prepared, preparedDedicatedSchema{
			resourceName: resource.Name,
			outputPath:   outputPath,
			data:         data,
		})
	}
	return prepared, nil
}

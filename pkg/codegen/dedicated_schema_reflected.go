// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/openchami/fabrica/pkg/annotations"
)

func (g *Generator) resolveDedicatedStorage(resource ResourceMetadata) (*annotations.ResolvedResourceStorage, error) {
	dialect, err := annotations.ParseDialect(g.DBDriver, annotations.SourcePosition{
		Filename: resource.SourcePath,
		TypeName: resource.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("parse dedicated schema dialect for %s: %w", resource.Name, err)
	}
	if resource.SourcePath != "" {
		resolved, resolveErr := annotations.ResolveStorageIntent(resource.SourcePath, resource.Name, dialect)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve dedicated schema storage for %s: %w", resource.Name, resolveErr)
		}
		return resolved, nil
	}

	fields := make([]annotations.ReflectedFieldStorage, 0, len(resource.SpecFields))
	for _, field := range resource.SpecFields {
		fields = append(fields, annotations.ReflectedFieldStorage{
			GoName: field.Name, JSONName: field.JSONName, GoType: field.GoType, Required: field.Required,
		})
	}
	resolved, err := annotations.ResolveStorageIntentFromReflect(resource.Name, fields, resource.Annotations, dialect)
	if err != nil {
		return nil, fmt.Errorf("resolve reflected dedicated schema storage for %s: %w", resource.Name, err)
	}
	return resolved, nil
}

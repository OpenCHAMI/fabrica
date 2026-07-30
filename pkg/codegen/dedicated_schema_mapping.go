// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/openchami/fabrica/pkg/annotations"
)

type dedicatedFieldMapping struct {
	GoName     string
	JSONName   string
	ColumnName string
}

var reservedDedicatedColumns = map[string]struct{}{
	"id":               {},
	"uid":              {},
	"name":             {},
	"namespace":        {},
	"api_version":      {},
	"kind":             {},
	"created_at":       {},
	"updated_at":       {},
	"resource_version": {},
	"status":           {},
	"labels":           {},
	"annotations":      {},
	"metadata":         {},
}

func buildDedicatedFieldMappings(fields []annotations.ResolvedFieldStorage) ([]dedicatedFieldMapping, error) {
	mappings := make([]dedicatedFieldMapping, 0, len(fields))
	jsonNames := make(map[string]string, len(fields))
	columns := make(map[string]string, len(fields))

	for _, field := range fields {
		if field.JSONName == "" {
			return nil, dedicatedFieldMappingError(field, "JSON field name is empty")
		}
		if firstField, exists := jsonNames[field.JSONName]; exists {
			return nil, dedicatedFieldMappingError(
				field,
				fmt.Sprintf("duplicate JSON name %q also used by %s", field.JSONName, firstField),
			)
		}

		normalized := normalizeDedicatedJSONName(field.JSONName)
		if normalized == "" {
			return nil, dedicatedFieldMappingError(field, fmt.Sprintf("JSON name %q has no valid column characters", field.JSONName))
		}
		columnName := "spec_" + normalized
		if _, reserved := reservedDedicatedColumns[columnName]; reserved {
			return nil, dedicatedFieldMappingError(field, fmt.Sprintf("normalized column %q is reserved", columnName))
		}
		if firstField, exists := columns[columnName]; exists {
			return nil, dedicatedFieldMappingError(
				field,
				fmt.Sprintf("duplicate normalized column %q also used by %s", columnName, firstField),
			)
		}

		jsonNames[field.JSONName] = field.GoName
		columns[columnName] = field.GoName
		mappings = append(mappings, dedicatedFieldMapping{
			GoName:     field.GoName,
			JSONName:   field.JSONName,
			ColumnName: columnName,
		})
	}

	return mappings, nil
}

func dedicatedFieldMappingError(field annotations.ResolvedFieldStorage, message string) error {
	source := field.Source
	return &annotations.CapabilityError{
		Filename:   source.Filename,
		Line:       source.Line,
		Column:     source.Column,
		TypeName:   source.TypeName,
		FieldName:  field.GoName,
		Directive:  "field mapping",
		Capability: annotations.CapabilityFieldType,
		Message:    message,
	}
}

func normalizeDedicatedJSONName(name string) string {
	runes := []rune(strings.TrimSpace(name))
	var normalized strings.Builder
	underscorePending := false

	for index, current := range runes {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			underscorePending = normalized.Len() > 0
			continue
		}

		if unicode.IsUpper(current) {
			previousIsLowerOrDigit := index > 0 && (unicode.IsLower(runes[index-1]) || unicode.IsDigit(runes[index-1]))
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			previousIsUpper := index > 0 && unicode.IsUpper(runes[index-1])
			underscorePending = underscorePending || normalized.Len() > 0 && (previousIsLowerOrDigit || previousIsUpper && nextIsLower)
		}
		if underscorePending && normalized.Len() > 0 {
			normalized.WriteByte('_')
		}
		normalized.WriteRune(unicode.ToLower(current))
		underscorePending = false
	}

	return strings.Trim(normalized.String(), "_")
}

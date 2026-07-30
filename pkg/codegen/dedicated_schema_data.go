// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/openchami/fabrica/pkg/annotations"
)

type dedicatedSchemaData struct {
	Name          string
	CopyrightYear int
	Fields        []dedicatedSchemaField
	Indexes       []dedicatedSchemaIndex
	NeedsEntSQL   bool
}

type dedicatedSchemaField struct {
	GoName         string
	JSONName       string
	ColumnName     string
	AccessorName   string
	Builder        string
	DefaultLiteral string
	HasDefault     bool
	Optional       bool
	Nillable       bool
	Immutable      bool
	Unique         bool
	Sensitive      bool
	Bcrypt         bool
	BcryptCost     int
	HashVariable   string
	ImportBcrypt   bool
}

func (g *Generator) dedicatedSchemaData(resource ResourceMetadata) (dedicatedSchemaData, error) {
	resolved, err := g.resolveDedicatedStorage(resource)
	if err != nil {
		return dedicatedSchemaData{}, err
	}
	data, err := buildDedicatedSchemaData(resolved, time.Now().UTC().Year())
	if err != nil {
		return dedicatedSchemaData{}, fmt.Errorf("build dedicated schema data for %s: %w", resource.Name, err)
	}
	return data, nil
}

func buildDedicatedSchemaData(resolved *annotations.ResolvedResourceStorage, year int) (dedicatedSchemaData, error) {
	if resolved == nil {
		return dedicatedSchemaData{}, fmt.Errorf("resolved dedicated schema storage is nil")
	}
	if resolved.Storage != annotations.ResourceStorageDedicated {
		return dedicatedSchemaData{}, fmt.Errorf("resource %s storage is not dedicated", resolved.Name)
	}
	if resolved.Dialect != annotations.DialectPostgreSQL && resolved.Dialect != annotations.DialectSQLite {
		return dedicatedSchemaData{}, fmt.Errorf("resource %s has unsupported resolved dialect", resolved.Name)
	}

	data := dedicatedSchemaData{
		Name:          resolved.Name,
		CopyrightYear: year,
		Fields:        make([]dedicatedSchemaField, 0, len(resolved.Fields)),
		Indexes:       make([]dedicatedSchemaIndex, 0),
	}
	mappings, err := buildDedicatedFieldMappings(resolved.Fields)
	if err != nil {
		return dedicatedSchemaData{}, err
	}
	bcryptImported := false
	for fieldIndex, resolvedField := range resolved.Fields {
		field, err := buildDedicatedSchemaField(resolvedField, mappings[fieldIndex])
		if err != nil {
			return dedicatedSchemaData{}, fmt.Errorf("field %s: %w", resolvedField.GoName, err)
		}
		if field.Bcrypt && !bcryptImported {
			field.ImportBcrypt = true
			bcryptImported = true
		}
		data.Fields = append(data.Fields, field)

		index, err := buildDedicatedSchemaIndex(resolvedField, field.ColumnName, resolved.Dialect)
		if err != nil {
			return dedicatedSchemaData{}, fmt.Errorf("field %s: %w", resolvedField.GoName, err)
		}
		if index != nil {
			data.Indexes = append(data.Indexes, *index)
			data.NeedsEntSQL = data.NeedsEntSQL || index.Method != ""
		}
	}
	return data, nil
}

func buildDedicatedSchemaField(
	resolved annotations.ResolvedFieldStorage,
	mapping dedicatedFieldMapping,
) (dedicatedSchemaField, error) {
	builder, err := dedicatedEntBuilder(resolved.Type.Kind)
	if err != nil {
		return dedicatedSchemaField{}, err
	}
	if err := validateDedicatedTransform(resolved); err != nil {
		return dedicatedSchemaField{}, err
	}

	field := dedicatedSchemaField{
		GoName:       mapping.GoName,
		JSONName:     mapping.JSONName,
		ColumnName:   mapping.ColumnName,
		AccessorName: dedicatedEntAccessor(mapping.ColumnName),
		Builder:      builder,
		Immutable:    resolved.Immutable,
		Unique:       resolved.Unique,
		Sensitive:    resolved.Sensitive,
		Bcrypt:       resolved.Transform.Kind == annotations.TransformBcrypt,
		BcryptCost:   resolved.Transform.BcryptCost,
		HashVariable: "hashed" + mapping.GoName,
	}
	switch resolved.Optionality {
	case annotations.OptionalityRequired:
	case annotations.OptionalityOptional:
		field.Optional = true
	case annotations.OptionalityNillable:
		field.Optional = true
		field.Nillable = true
	case annotations.OptionalityUnknown:
		return dedicatedSchemaField{}, fmt.Errorf("optionality is unknown")
	default:
		return dedicatedSchemaField{}, fmt.Errorf("optionality is unsupported")
	}

	if resolved.Default != nil {
		literal, literalErr := dedicatedDefaultLiteral(resolved.Type.Kind, resolved.Default)
		if literalErr != nil {
			return dedicatedSchemaField{}, literalErr
		}
		field.DefaultLiteral = literal
		field.HasDefault = true
		if !resolved.Type.Pointer() {
			field.Optional = false
			field.Nillable = false
		}
	}
	return field, nil
}

func dedicatedEntAccessor(columnName string) string {
	parts := strings.Split(columnName, "_")
	for index, part := range parts {
		switch strings.ToLower(part) {
		case "id":
			parts[index] = "ID"
		case "uid":
			parts[index] = "UID"
		case "url":
			parts[index] = "URL"
		default:
			parts[index] = cases.Title(language.English).String(part)
		}
	}
	return strings.Join(parts, "")
}

func dedicatedEntBuilder(kind annotations.FieldKind) (string, error) {
	switch kind {
	case annotations.FieldKindString:
		return "String", nil
	case annotations.FieldKindBool:
		return "Bool", nil
	case annotations.FieldKindInt:
		return "Int", nil
	case annotations.FieldKindInt64:
		return "Int64", nil
	case annotations.FieldKindFloat64:
		return "Float", nil
	case annotations.FieldKindTime:
		return "Time", nil
	case annotations.FieldKindStringSlice:
		return "Strings", nil
	case annotations.FieldKindUnknown:
		return "", fmt.Errorf("field type is unknown")
	default:
		return "", fmt.Errorf("field type is unsupported")
	}
}

func validateDedicatedTransform(field annotations.ResolvedFieldStorage) error {
	switch field.Transform.Kind {
	case annotations.TransformStandard:
		return nil
	case annotations.TransformBcrypt:
		if field.Type.Kind != annotations.FieldKindString {
			return fmt.Errorf("bcrypt storage requires a string field")
		}
		return nil
	case annotations.TransformUnknown:
		return fmt.Errorf("storage transform is unknown")
	default:
		return fmt.Errorf("storage transform is unsupported")
	}
}

func dedicatedDefaultLiteral(kind annotations.FieldKind, value annotations.DefaultValue) (string, error) {
	compatible := false
	switch value.(type) {
	case annotations.StringDefault:
		compatible = kind == annotations.FieldKindString
	case annotations.BoolDefault:
		compatible = kind == annotations.FieldKindBool
	case annotations.IntDefault:
		compatible = kind == annotations.FieldKindInt
	case annotations.Int64Default:
		compatible = kind == annotations.FieldKindInt64
	case annotations.Float64Default:
		compatible = kind == annotations.FieldKindFloat64
	default:
		return "", fmt.Errorf("default variant is unsupported")
	}
	if !compatible {
		return "", fmt.Errorf("default variant does not match field type")
	}
	literal, err := annotations.GoLiteral(value)
	if err != nil {
		return "", fmt.Errorf("render typed default: %w", err)
	}
	return literal, nil
}

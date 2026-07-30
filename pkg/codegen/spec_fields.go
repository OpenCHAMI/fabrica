// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"reflect"
	"strings"
)

// SpecField describes an exported resource Spec field used by code-generation templates.
type SpecField struct {
	Name         string
	JSONName     string
	Type         string
	GoType       reflect.Type
	Required     bool
	ExampleValue string
}

func extractSpecFields(resourceType reflect.Type) []SpecField {
	var fields []SpecField
	for i := 0; i < resourceType.NumField(); i++ {
		field := resourceType.Field(i)
		if field.Name != "Spec" {
			continue
		}
		specType := field.Type
		if specType.Kind() == reflect.Pointer {
			specType = specType.Elem()
		}
		for j := 0; j < specType.NumField(); j++ {
			specField := specType.Field(j)
			if !specField.IsExported() {
				continue
			}
			jsonName := specField.Name
			parts := strings.Split(specField.Tag.Get("json"), ",")
			if parts[0] != "" && parts[0] != "-" {
				jsonName = parts[0]
			}
			fields = append(fields, SpecField{
				Name:         specField.Name,
				JSONName:     jsonName,
				Type:         specField.Type.String(),
				GoType:       specField.Type,
				Required:     strings.Contains(specField.Tag.Get("validate"), "required"),
				ExampleValue: generateExampleValue(specField.Type, specField.Name),
			})
		}
		break
	}
	return fields
}

func generateExampleValue(fieldType reflect.Type, fieldName string) string {
	switch fieldType.Kind() {
	case reflect.String:
		lowerName := strings.ToLower(fieldName)
		switch {
		case strings.Contains(lowerName, "name"):
			return "example-name"
		case strings.Contains(lowerName, "description"):
			return "Example description"
		case strings.Contains(lowerName, "email"):
			return "user@example.com"
		case strings.Contains(lowerName, "url"), strings.Contains(lowerName, "uri"):
			return "https://example.com"
		case strings.Contains(lowerName, "ip"), strings.Contains(lowerName, "address"):
			return "192.168.1.1"
		case strings.Contains(lowerName, "location"):
			return "DataCenter A"
		default:
			return "example-value"
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "42"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "42"
	case reflect.Float32, reflect.Float64:
		return "3.14"
	case reflect.Bool:
		return "true"
	case reflect.Slice:
		if fieldType.Elem().Kind() == reflect.String {
			return `["item1","item2"]`
		}
		return "[]"
	case reflect.Map:
		return `{"key":"value"}`
	default:
		return `{}`
	}
}

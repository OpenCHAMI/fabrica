// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/openchami/fabrica/pkg/annotations"
)

var templateFuncs = template.FuncMap{
	"toLower":    strings.ToLower,
	"toUpper":    strings.ToUpper,
	"title":      cases.Title(language.English).String,
	"trimPrefix": strings.TrimPrefix,
	"goLiteral":  annotations.GoLiteral,
	"isDedicatedStorage": func(resource ResourceMetadata) bool {
		return resource.Annotations != nil && resource.Annotations.StorageMode == annotations.StorageModeDedicated
	},
	"isDedicatedAnnotations": func(resourceAnnotations *annotations.ResourceAnnotations) bool {
		return resourceAnnotations != nil && resourceAnnotations.StorageMode == annotations.StorageModeDedicated
	},
	"hasGenericStorage": func(resources []ResourceMetadata) bool {
		for _, resource := range resources {
			if resource.Annotations == nil || resource.Annotations.StorageMode != annotations.StorageModeDedicated {
				return true
			}
		}
		return false
	},
	"hasDedicatedStorage": func(resources []ResourceMetadata) bool {
		for _, resource := range resources {
			if resource.Annotations != nil && resource.Annotations.StorageMode == annotations.StorageModeDedicated {
				return true
			}
		}
		return false
	},
	"replace": func(old, newStr, s string) string {
		return strings.ReplaceAll(s, old, newStr)
	},
	"split": func(sep, s string) []string {
		return strings.Split(s, sep)
	},
	"last": func(s []string) string {
		if len(s) == 0 {
			return ""
		}
		return s[len(s)-1]
	},
	"camelCase": func(s string) string {
		if len(s) == 0 {
			return s
		}
		return strings.ToLower(s[:1]) + s[1:]
	},
	"specToJSON": func(fields []SpecField) string {
		if len(fields) == 0 {
			return `{"name": "example"}`
		}

		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			value := formatJSONValue(field.Type, field.ExampleValue)
			parts = append(parts, fmt.Sprintf(`"%s": %s`, field.JSONName, value))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	},
	"specToJSONPretty": func(fields []SpecField) string {
		if len(fields) == 0 {
			return `{
    "name": "example"
  }`
		}

		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			value := formatJSONValue(field.Type, field.ExampleValue)
			parts = append(parts, fmt.Sprintf(`    "%s": %s`, field.JSONName, value))
		}
		return "{\n" + strings.Join(parts, ",\n") + "\n  }"
	},
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"
	"strings"
)

// Schema Builders

// schemaObject creates a basic JSON Schema object with properties.
func schemaObject(props map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
}

// schemaObjectWithRequired creates a JSON Schema object with required fields.
func schemaObjectWithRequired(props map[string]interface{}, required []string) map[string]interface{} {
	s := schemaObject(props)
	s["required"] = required
	return s
}

// modeField creates the schema for a "mode" parameter (dry_run or execute).
func modeField() map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"enum":        []string{"dry_run", "execute"},
		"description": "Operation mode. Defaults to dry_run for mutating tools.",
	}
}

// fieldArraySchema creates the schema for an array of resource field definitions.
func fieldArraySchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "array",
		"items": schemaObjectWithRequired(map[string]interface{}{
			"name":        map[string]interface{}{"type": "string"},
			"type":        map[string]interface{}{"type": "string"},
			"json_name":   map[string]interface{}{"type": "string"},
			"required":    map[string]interface{}{"type": "boolean"},
			"validation":  map[string]interface{}{"type": "string"},
			"description": map[string]interface{}{"type": "string"},
		}, []string{"name", "type"}),
	}
}

// Validation Functions

// validateMCPToolArgs validates arguments against tool schemas.
// Returns a toolError if any argument is invalid.
func validateMCPToolArgs(tool string, args map[string]interface{}) error {
	schemas := toolArgSchemas()
	schema, ok := schemas[tool]
	if !ok {
		return nil // No validation required for this tool
	}

	for key, value := range args {
		kind, ok := schema[key]
		if !ok {
			return toolError("invalid_arguments", "unknown argument "+key, "Remove unsupported arguments and retry", nil)
		}
		if err := validateMCPArgType(key, value, kind); err != nil {
			return err
		}
	}
	return nil
}

// validateMCPArgType validates a single argument value against its expected type.
func validateMCPArgType(key string, value interface{}, kind string) error {
	switch kind {
	case "string":
		if _, ok := value.(string); !ok {
			return toolError("invalid_arguments", key+" must be a string", "Pass a string value", nil)
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return toolError("invalid_arguments", key+" must be a boolean", "Pass true or false", nil)
		}
	case "number":
		switch value.(type) {
		case float64, int:
		default:
			return toolError("invalid_arguments", key+" must be a number", "Pass a numeric value", nil)
		}
	case "mode":
		if s, ok := value.(string); !ok || (s != "dry_run" && s != "execute") {
			return toolError("invalid_arguments", key+" must be dry_run or execute", "Pass mode as dry_run or execute", nil)
		}
	case "events_bus":
		if s, ok := value.(string); !ok || s != "memory" {
			return toolError("invalid_arguments", key+" must be memory", "Use events_bus=memory", nil)
		}
	case "storage_type":
		if s, ok := value.(string); !ok || (s != "file" && s != "ent" && s != "custom") {
			return toolError("invalid_arguments", key+" must be file, ent, or custom", "Pass storage_type as file, ent, or custom", nil)
		}
	case "db":
		if s, ok := value.(string); !ok || (s != "sqlite" && s != "postgres" && s != "mysql") {
			return toolError("invalid_arguments", key+" must be sqlite, postgres, or mysql", "Pass a supported database driver", nil)
		}
	case "validation_mode":
		if s, ok := value.(string); !ok || (s != "strict" && s != "warn" && s != "disabled") {
			return toolError("invalid_arguments", key+" must be strict, warn, or disabled", "Pass a supported validation mode", nil)
		}
	case "workflow_goal":
		if s, ok := value.(string); !ok || (s != "new_crud_api" && s != "add_resource" && s != "verify_project") {
			return toolError("invalid_arguments", key+" must be a supported workflow goal", "Use new_crud_api, add_resource, or verify_project", nil)
		}
	case "artifacts":
		items, ok := value.([]interface{})
		if !ok {
			return toolError("invalid_arguments", key+" must be an array", "Pass artifacts as an array of strings", nil)
		}
		for _, item := range items {
			s, ok := item.(string)
			if !ok || (s != "handlers" && s != "storage" && s != "client" && s != "openapi") {
				return toolError("invalid_arguments", "unsupported artifact "+fmt.Sprintf("%v", item), "Use handlers, storage, client, or openapi", nil)
			}
		}
	case "string_array":
		switch items := value.(type) {
		case []interface{}:
			for _, item := range items {
				if _, ok := item.(string); !ok {
					return toolError("invalid_arguments", key+" must contain only strings", "Pass an array of strings", nil)
				}
			}
		case []string:
			return nil
		default:
			return toolError("invalid_arguments", key+" must be an array", "Pass an array of strings", nil)
		}
	case "field_array":
		if _, err := parseMCPFields(value); err != nil {
			return toolError("invalid_arguments", key+" is invalid", "Pass an array of field objects with name and type", err)
		}
	}
	return nil
}

// toolArgSchemas defines the expected argument types for each tool.
// Format: toolName -> argName -> argType
func toolArgSchemas() map[string]map[string]string {
	return map[string]map[string]string{
		"inspect_project":  {"project_path": "string"},
		"validate_project": {"project_path": "string"},
		"create_service": {
			"project_name": "string", "mode": "mode", "target_dir": "string",
			"module": "string", "description": "string", "group": "string",
			"storage_version": "string", "versions": "string_array",
			"auth": "bool", "metrics": "bool", "events": "bool",
			"events_bus": "events_bus", "reconcile": "bool",
			"reconcile_workers": "number", "reconcile_requeue": "number",
			"storage_type": "storage_type", "db": "db", "validation_mode": "validation_mode",
		},
		"add_resource": {
			"resource_name": "string", "project_path": "string", "mode": "mode",
			"version": "string", "with_validation": "bool", "with_status": "bool",
			"with_versioning": "bool", "package": "string", "force": "bool",
		},
		"define_resource_schema": {
			"resource_name": "string", "project_path": "string", "version": "string",
			"mode": "mode", "spec_fields": "field_array", "status_fields": "field_array",
		},
		"add_version": {
			"new_version": "string", "project_path": "string", "mode": "mode",
			"from": "string", "force": "bool",
		},
		"generate_code": {
			"project_path": "string", "mode": "mode", "artifacts": "artifacts",
			"force": "bool", "debug": "bool", "fabrica_source": "string",
		},
		"sync_dependencies": {"project_path": "string", "mode": "mode"},
		"build_project":     {"project_path": "string", "mode": "mode", "packages": "string_array"},
		"test_project":      {"project_path": "string", "mode": "mode", "packages": "string_array"},
		"smoke_test_api": {
			"project_path": "string", "mode": "mode", "base_url": "string",
			"start_server": "bool", "timeout_seconds": "number", "server_arguments": "string_array",
		},
		"describe_workflow": {"goal": "workflow_goal"},
	}
}

// parseMCPFields converts raw JSON data into mcpResourceField structs.
// Validates that all fields have required name and type properties.
func parseMCPFields(raw interface{}) ([]mcpResourceField, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array")
	}
	fields := make([]mcpResourceField, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("field %d must be an object", i)
		}
		name := strings.TrimSpace(getString(m, "name", ""))
		fieldType := strings.TrimSpace(getString(m, "type", ""))
		if name == "" || fieldType == "" {
			return nil, fmt.Errorf("field %d requires name and type", i)
		}
		jsonName := strings.TrimSpace(getString(m, "json_name", ""))
		if jsonName == "" {
			jsonName = lowerFirst(name)
		}
		fields = append(fields, mcpResourceField{
			Name:        name,
			Type:        fieldType,
			JSONName:    jsonName,
			Required:    getBool(m, "required", false),
			Validation:  strings.TrimSpace(getString(m, "validation", "")),
			Description: strings.TrimSpace(getString(m, "description", "")),
		})
	}
	return fields, nil
}

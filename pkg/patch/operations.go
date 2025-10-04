// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package patch provides support for HTTP PATCH operations including
// RFC 7386 (JSON Merge Patch), RFC 6902 (JSON Patch), and shorthand patches.
//
// This package enables partial updates to resources using standardized
// PATCH operations with automatic validation and conflict detection.
//
// Usage:
//
//	// Apply JSON Merge Patch
//	updated, err := patch.ApplyMergePatch(original, patchDoc)
//
//	// Apply JSON Patch
//	updated, err := patch.ApplyJSONPatch(original, operations)
//
//	// Use middleware
//	handler := patch.PatchMiddleware(myHandler)
package patch

import (
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// PatchType represents the type of PATCH operation
//
//nolint:revive // "PatchType" name is intentional; "Type" alone would be ambiguous
type PatchType string

const (
	// JSONMergePatch represents RFC 7386 JSON Merge Patch
	JSONMergePatch PatchType = "application/merge-patch+json"

	// JSONPatch represents RFC 6902 JSON Patch
	JSONPatch PatchType = "application/json-patch+json"

	// ShorthandPatch represents a simplified patch format
	ShorthandPatch PatchType = "application/shorthand-patch+json"

	// StrategicMergePatch represents Kubernetes-style strategic merge patch
	StrategicMergePatch PatchType = "application/strategic-merge-patch+json"
)

// Operation represents a JSON Patch operation (RFC 6902)
type Operation struct {
	Op    string      `json:"op"`              // Operation: add, remove, replace, move, copy, test
	Path  string      `json:"path"`            // JSON Pointer path
	Value interface{} `json:"value,omitempty"` // Value for add, replace, test operations
	From  string      `json:"from,omitempty"`  // Source path for move, copy operations
}

// PatchRequest represents a PATCH request with metadata
//
//nolint:revive // "PatchRequest" name is intentional; "Request" alone would be ambiguous
type PatchRequest struct {
	Type      PatchType       `json:"type"`
	Patch     json.RawMessage `json:"patch"`
	DryRun    bool            `json:"dryRun,omitempty"`
	FieldMask []string        `json:"fieldMask,omitempty"` // Limit patch to specific fields
}

// PatchResult contains the result of a patch operation
//
//nolint:revive // "PatchResult" name is intentional; "Result" alone would be ambiguous
type PatchResult struct {
	Modified bool            `json:"modified"`
	Original json.RawMessage `json:"original,omitempty"`
	Updated  json.RawMessage `json:"updated"`
	Changes  []string        `json:"changes,omitempty"` // List of changed paths
}

// ApplyMergePatch applies a JSON Merge Patch (RFC 7386)
// This is the simplest form - just merge the patch into the original
func ApplyMergePatch(original, patch []byte) ([]byte, error) {
	if len(original) == 0 {
		return nil, fmt.Errorf("original document is empty")
	}

	if len(patch) == 0 {
		return original, nil
	}

	// Validate both are valid JSON
	if !json.Valid(original) {
		return nil, fmt.Errorf("original document is not valid JSON")
	}

	if !json.Valid(patch) {
		return nil, fmt.Errorf("patch document is not valid JSON")
	}

	// Use json-patch library for merge patch
	merged, err := jsonpatch.MergePatch(original, patch)
	if err != nil {
		return nil, fmt.Errorf("failed to apply merge patch: %w", err)
	}

	return merged, nil
}

// ApplyJSONPatch applies a JSON Patch (RFC 6902)
func ApplyJSONPatch(original, patch []byte) ([]byte, error) {
	if len(original) == 0 {
		return nil, fmt.Errorf("original document is empty")
	}

	if len(patch) == 0 {
		return original, nil
	}

	// Parse patch operations
	patchObj, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON Patch: %w", err)
	}

	// Apply patch
	modified, err := patchObj.Apply(original)
	if err != nil {
		return nil, fmt.Errorf("failed to apply JSON Patch: %w", err)
	}

	return modified, nil
}

// ApplyShorthandPatch applies a simplified patch format
// Shorthand format: {"field.path": "value", "other.field": null}
// null values remove the field
func ApplyShorthandPatch(original, patch []byte) ([]byte, error) {
	// Parse shorthand into standard operations
	var shorthand map[string]interface{}
	if err := json.Unmarshal(patch, &shorthand); err != nil {
		return nil, fmt.Errorf("invalid shorthand patch: %w", err)
	}

	// Parse original to check which paths exist
	var originalDoc interface{}
	if err := json.Unmarshal(original, &originalDoc); err != nil {
		return nil, fmt.Errorf("failed to parse original document: %w", err)
	}

	// Convert shorthand to JSON Patch operations
	var ops []Operation
	for path, value := range shorthand {
		// Convert dot notation to JSON Pointer
		pointer := "/" + strings.ReplaceAll(path, ".", "/")

		if value == nil {
			// null means remove
			ops = append(ops, Operation{
				Op:   "remove",
				Path: pointer,
			})
		} else {
			// Check if path exists in original - use add if not, replace if yes
			exists := pathExists(originalDoc, strings.Split(path, "."))
			op := "replace"
			if !exists {
				op = "add"
			}
			ops = append(ops, Operation{
				Op:    op,
				Path:  pointer,
				Value: value,
			})
		}
	}

	// Convert operations to JSON and apply as JSON Patch
	opsJSON, err := json.Marshal(ops)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal operations: %w", err)
	}

	return ApplyJSONPatch(original, opsJSON)
}

// pathExists checks if a path exists in a document
func pathExists(doc interface{}, path []string) bool {
	if len(path) == 0 {
		return true
	}

	switch v := doc.(type) {
	case map[string]interface{}:
		if val, ok := v[path[0]]; ok {
			if len(path) == 1 {
				return true
			}
			return pathExists(val, path[1:])
		}
	case []interface{}:
		// Array index - not supporting in shorthand for simplicity
		return false
	}

	return false
}

// DetectPatchType determines the patch type from Content-Type header
func DetectPatchType(contentType string) PatchType {
	contentType = strings.ToLower(strings.TrimSpace(contentType))

	// Remove charset and other parameters
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	switch contentType {
	case string(JSONMergePatch):
		return JSONMergePatch
	case string(JSONPatch):
		return JSONPatch
	case string(ShorthandPatch):
		return ShorthandPatch
	case string(StrategicMergePatch):
		return StrategicMergePatch
	default:
		// Default to JSON Merge Patch for standard application/json
		return JSONMergePatch
	}
}

// ApplyPatch applies the appropriate patch based on the patch type
func ApplyPatch(original []byte, patchData []byte, patchType PatchType) ([]byte, error) {
	switch patchType {
	case JSONMergePatch:
		return ApplyMergePatch(original, patchData)
	case JSONPatch:
		return ApplyJSONPatch(original, patchData)
	case ShorthandPatch:
		return ApplyShorthandPatch(original, patchData)
	default:
		return nil, fmt.Errorf("unsupported patch type: %s", patchType)
	}
}

// ValidateJSONPatch validates JSON Patch operations
func ValidateJSONPatch(patch []byte) error {
	var ops []Operation
	if err := json.Unmarshal(patch, &ops); err != nil {
		return fmt.Errorf("invalid JSON Patch format: %w", err)
	}

	validOps := map[string]bool{
		"add": true, "remove": true, "replace": true,
		"move": true, "copy": true, "test": true,
	}

	for i, op := range ops {
		if !validOps[op.Op] {
			return fmt.Errorf("invalid operation at index %d: %s", i, op.Op)
		}

		if op.Path == "" {
			return fmt.Errorf("missing path in operation at index %d", i)
		}

		// Validate path is a valid JSON Pointer
		if !strings.HasPrefix(op.Path, "/") && op.Path != "" {
			return fmt.Errorf("invalid JSON Pointer path at index %d: %s", i, op.Path)
		}

		// Validate required fields for specific operations
		switch op.Op {
		case "add", "replace", "test":
			if op.Value == nil {
				return fmt.Errorf("missing value for %s operation at index %d", op.Op, i)
			}
		case "move", "copy":
			if op.From == "" {
				return fmt.Errorf("missing from field for %s operation at index %d", op.Op, i)
			}
		}
	}

	return nil
}

// ComputePatchChanges computes the list of changed paths
func ComputePatchChanges(original, updated []byte) ([]string, error) {
	// Create a patch between original and updated
	patch, err := jsonpatch.CreateMergePatch(original, updated)
	if err != nil {
		return nil, fmt.Errorf("failed to compute changes: %w", err)
	}

	// Parse the patch to extract changed paths
	var changes map[string]interface{}
	if err := json.Unmarshal(patch, &changes); err != nil {
		return nil, fmt.Errorf("failed to parse patch: %w", err)
	}

	var paths []string
	extractPaths(changes, "", &paths)
	return paths, nil
}

// extractPaths recursively extracts all paths from a nested map
func extractPaths(obj interface{}, prefix string, paths *[]string) {
	switch v := obj.(type) {
	case map[string]interface{}:
		for key, val := range v {
			path := prefix + "/" + key
			*paths = append(*paths, path)
			extractPaths(val, path, paths)
		}
	case []interface{}:
		for i, val := range v {
			path := fmt.Sprintf("%s/%d", prefix, i)
			*paths = append(*paths, path)
			extractPaths(val, path, paths)
		}
	}
}

// CreatePatch creates a JSON Patch from old and new versions
func CreatePatch(original, updated []byte) ([]byte, error) {
	return jsonpatch.CreateMergePatch(original, updated)
}

// TestOperation tests if a JSON Pointer path has a specific value
func TestOperation(doc []byte, path string, expectedValue interface{}) (bool, error) {
	ops := []Operation{{
		Op:    "test",
		Path:  path,
		Value: expectedValue,
	}}

	opsJSON, err := json.Marshal(ops)
	if err != nil {
		return false, err
	}

	_, err = ApplyJSONPatch(doc, opsJSON)
	return err == nil, nil
}

// PatchOptions defines options for patch operations
//
//nolint:revive // "PatchOptions" name is intentional; "Options" alone would be ambiguous
type PatchOptions struct {
	DryRun            bool     // Don't actually modify, just validate
	FieldMask         []string // Only allow patching specific fields
	RequireETag       bool     // Require ETag for optimistic concurrency
	AllowAddFields    bool     // Allow adding new fields
	AllowRemoveFields bool     // Allow removing fields
}

// ApplyPatchWithOptions applies a patch with additional constraints
func ApplyPatchWithOptions(original, patch []byte, patchType PatchType, opts PatchOptions) (*PatchResult, error) {
	result := &PatchResult{
		Original: original,
	}

	// Apply the patch
	updated, err := ApplyPatch(original, patch, patchType)
	if err != nil {
		return nil, err
	}

	// Compute changes
	changes, err := ComputePatchChanges(original, updated)
	if err == nil {
		result.Changes = changes
	}

	// Check field mask if specified
	if len(opts.FieldMask) > 0 {
		// Validate that all changes are within the field mask
		for _, change := range result.Changes {
			allowed := false
			for _, mask := range opts.FieldMask {
				if strings.HasPrefix(change, "/"+mask) {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, fmt.Errorf("field %s is not in the field mask", change)
			}
		}
	}

	result.Modified = len(result.Changes) > 0
	if !opts.DryRun {
		result.Updated = updated
	} else {
		result.Updated = original
	}

	return result, nil
}

// MergePatchFromMap creates a merge patch from a map of changes
func MergePatchFromMap(changes map[string]interface{}) ([]byte, error) {
	return json.Marshal(changes)
}

// JSONPatchFromOperations creates a JSON Patch from a list of operations
func JSONPatchFromOperations(ops []Operation) ([]byte, error) {
	return json.Marshal(ops)
}

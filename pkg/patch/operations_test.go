// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package patch

import (
	"encoding/json"
	"testing"
)

func TestApplyMergePatch(t *testing.T) {
	original := []byte(`{"name":"John","age":30,"city":"NYC"}`)
	patch := []byte(`{"age":31,"city":"SF"}`)

	result, err := ApplyMergePatch(original, patch)
	if err != nil {
		t.Fatalf("ApplyMergePatch failed: %v", err)
	}

	var merged map[string]interface{}
	if err := json.Unmarshal(result, &merged); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if merged["name"] != "John" {
		t.Error("name should remain unchanged")
	}
	if merged["age"] != float64(31) {
		t.Errorf("age should be 31, got %v", merged["age"])
	}
	if merged["city"] != "SF" {
		t.Errorf("city should be SF, got %v", merged["city"])
	}
}

func TestApplyMergePatch_NullRemoves(t *testing.T) {
	original := []byte(`{"name":"John","age":30,"city":"NYC"}`)
	patch := []byte(`{"city":null}`)

	result, err := ApplyMergePatch(original, patch)
	if err != nil {
		t.Fatalf("ApplyMergePatch failed: %v", err)
	}

	var merged map[string]interface{}
	if err := json.Unmarshal(result, &merged); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if _, exists := merged["city"]; exists {
		t.Error("city should be removed")
	}
	if merged["name"] != "John" {
		t.Error("name should remain")
	}
}

func TestApplyJSONPatch(t *testing.T) {
	original := []byte(`{"name":"John","age":30}`)
	patch := []byte(`[
		{"op":"replace","path":"/age","value":31},
		{"op":"add","path":"/city","value":"SF"}
	]`)

	result, err := ApplyJSONPatch(original, patch)
	if err != nil {
		t.Fatalf("ApplyJSONPatch failed: %v", err)
	}

	var patched map[string]interface{}
	if err := json.Unmarshal(result, &patched); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if patched["age"] != float64(31) {
		t.Errorf("age should be 31, got %v", patched["age"])
	}
	if patched["city"] != "SF" {
		t.Errorf("city should be SF, got %v", patched["city"])
	}
}

func TestApplyJSONPatch_Remove(t *testing.T) {
	original := []byte(`{"name":"John","age":30,"city":"NYC"}`)
	patch := []byte(`[{"op":"remove","path":"/city"}]`)

	result, err := ApplyJSONPatch(original, patch)
	if err != nil {
		t.Fatalf("ApplyJSONPatch failed: %v", err)
	}

	var patched map[string]interface{}
	if err := json.Unmarshal(result, &patched); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if _, exists := patched["city"]; exists {
		t.Error("city should be removed")
	}
}

func TestApplyShorthandPatch(t *testing.T) {
	original := []byte(`{"user":{"name":"John","age":30}}`)
	patch := []byte(`{"user.age":31,"user.city":"SF"}`)

	result, err := ApplyShorthandPatch(original, patch)
	if err != nil {
		t.Fatalf("ApplyShorthandPatch failed: %v", err)
	}

	var patched map[string]interface{}
	if err := json.Unmarshal(result, &patched); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	user := patched["user"].(map[string]interface{})
	if user["age"] != float64(31) {
		t.Errorf("age should be 31, got %v", user["age"])
	}
}

func TestDetectPatchType(t *testing.T) {
	tests := []struct {
		contentType string
		expected    PatchType
	}{
		{"application/merge-patch+json", JSONMergePatch},
		{"application/json-patch+json", JSONPatch},
		{"application/shorthand-patch+json", ShorthandPatch},
		{"application/json", JSONMergePatch}, // Default
		{"application/json; charset=utf-8", JSONMergePatch},
	}

	for _, test := range tests {
		result := DetectPatchType(test.contentType)
		if result != test.expected {
			t.Errorf("DetectPatchType(%q) = %q, want %q", test.contentType, result, test.expected)
		}
	}
}

func TestValidateJSONPatch(t *testing.T) {
	validPatch := []byte(`[
		{"op":"add","path":"/name","value":"John"},
		{"op":"remove","path":"/age"},
		{"op":"replace","path":"/city","value":"SF"}
	]`)

	if err := ValidateJSONPatch(validPatch); err != nil {
		t.Errorf("Valid patch should not error: %v", err)
	}

	invalidOp := []byte(`[{"op":"invalid","path":"/name"}]`)
	if err := ValidateJSONPatch(invalidOp); err == nil {
		t.Error("Invalid operation should error")
	}

	missingPath := []byte(`[{"op":"add","value":"test"}]`)
	if err := ValidateJSONPatch(missingPath); err == nil {
		t.Error("Missing path should error")
	}

	missingValue := []byte(`[{"op":"add","path":"/name"}]`)
	if err := ValidateJSONPatch(missingValue); err == nil {
		t.Error("Missing value for add should error")
	}
}

func TestComputePatchChanges(t *testing.T) {
	original := []byte(`{"name":"John","age":30}`)
	updated := []byte(`{"name":"John","age":31,"city":"SF"}`)

	changes, err := ComputePatchChanges(original, updated)
	if err != nil {
		t.Fatalf("ComputePatchChanges failed: %v", err)
	}

	if len(changes) == 0 {
		t.Error("Should detect changes")
	}

	// Check that age and city are in changes
	hasAge := false
	hasCity := false
	for _, path := range changes {
		if path == "/age" {
			hasAge = true
		}
		if path == "/city" {
			hasCity = true
		}
	}

	if !hasAge {
		t.Error("Should detect age change")
	}
	if !hasCity {
		t.Error("Should detect city addition")
	}
}

func TestCreatePatch(t *testing.T) {
	original := []byte(`{"name":"John","age":30}`)
	updated := []byte(`{"name":"John","age":31}`)

	patch, err := CreatePatch(original, updated)
	if err != nil {
		t.Fatalf("CreatePatch failed: %v", err)
	}

	// Apply the patch back
	result, err := ApplyMergePatch(original, patch)
	if err != nil {
		t.Fatalf("Failed to apply created patch: %v", err)
	}

	// Result should match updated
	var resultMap, updatedMap map[string]interface{}
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if err := json.Unmarshal(updated, &updatedMap); err != nil {
		t.Fatalf("Failed to unmarshal updated: %v", err)
	}

	if resultMap["age"] != updatedMap["age"] {
		t.Error("Patch application should produce same result")
	}
}

func TestApplyPatchWithOptions_FieldMask(t *testing.T) {
	original := []byte(`{"name":"John","age":30,"city":"NYC"}`)
	patch := []byte(`{"age":31,"city":"SF"}`)

	opts := PatchOptions{
		FieldMask: []string{"age"}, // Only allow patching age
	}

	_, err := ApplyPatchWithOptions(original, patch, JSONMergePatch, opts)
	if err == nil {
		t.Error("Should error when patching field outside mask")
	}

	// Should succeed when only patching allowed field
	patch2 := []byte(`{"age":31}`)
	result, err := ApplyPatchWithOptions(original, patch2, JSONMergePatch, opts)
	if err != nil {
		t.Errorf("Should allow patching field in mask: %v", err)
	}

	if !result.Modified {
		t.Error("Should be marked as modified")
	}
}

func TestApplyPatchWithOptions_DryRun(t *testing.T) {
	original := []byte(`{"name":"John","age":30}`)
	patch := []byte(`{"age":31}`)

	opts := PatchOptions{
		DryRun: true,
	}

	result, err := ApplyPatchWithOptions(original, patch, JSONMergePatch, opts)
	if err != nil {
		t.Fatalf("DryRun patch failed: %v", err)
	}

	if !result.Modified {
		t.Error("Should detect modification in dry run")
	}

	// Updated should be same as original in dry run
	if string(result.Updated) != string(original) {
		t.Error("DryRun should not modify data")
	}

	if len(result.Changes) == 0 {
		t.Error("Should report changes in dry run")
	}
}

func TestTestOperation(t *testing.T) {
	doc := []byte(`{"name":"John","age":30}`)

	// Test matching value
	matches, err := TestOperation(doc, "/name", "John")
	if err != nil || !matches {
		t.Error("Test should pass for matching value")
	}

	// Test non-matching value
	matches, _ = TestOperation(doc, "/name", "Jane")
	if matches {
		t.Error("Test should fail for non-matching value")
	}
}

func TestMergePatchFromMap(t *testing.T) {
	changes := map[string]interface{}{
		"age":  31,
		"city": "SF",
	}

	patch, err := MergePatchFromMap(changes)
	if err != nil {
		t.Fatalf("MergePatchFromMap failed: %v", err)
	}

	var patchMap map[string]interface{}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		t.Fatalf("Failed to unmarshal patch: %v", err)
	}

	if patchMap["age"] != float64(31) {
		t.Error("Patch should contain age")
	}
	if patchMap["city"] != "SF" {
		t.Error("Patch should contain city")
	}
}

func TestJSONPatchFromOperations(t *testing.T) {
	ops := []Operation{
		{Op: "replace", Path: "/age", Value: 31},
		{Op: "add", Path: "/city", Value: "SF"},
	}

	patch, err := JSONPatchFromOperations(ops)
	if err != nil {
		t.Fatalf("JSONPatchFromOperations failed: %v", err)
	}

	var patchOps []Operation
	if err := json.Unmarshal(patch, &patchOps); err != nil {
		t.Fatalf("Failed to unmarshal operations: %v", err)
	}

	if len(patchOps) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(patchOps))
	}

	if patchOps[0].Op != "replace" {
		t.Error("First operation should be replace")
	}
}

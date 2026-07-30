// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateEntSchemasDedicated(t *testing.T) {
	schemaStr := generateDedicatedShapeSchema(t, "postgres").content

	if !strings.Contains(schemaStr, "type DedicatedShape struct") {
		t.Error("expected dedicated schema type definition")
	}

	if strings.Contains(schemaStr, "bcrypt") {
		t.Error("did not expect bcrypt implementation in schema")
	}

	if !strings.Contains(schemaStr, "Sensitive()") {
		t.Error("expected Sensitive() for Value field")
	}

	if !strings.Contains(schemaStr, "Immutable()") {
		t.Error("expected Immutable() for Value field")
	}

	if !strings.Contains(schemaStr, "Unique()") {
		t.Error("expected Unique() for Name field")
	}

	if strings.Contains(schemaStr, "index.Fields(\"spec_code\")") {
		t.Error("did not expect redundant B-tree index on unique code field")
	}
}

func TestGenerateEntSchemasGenericOnly(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change to temp directory: %v", err)
	}

	gen := NewGenerator(tmpDir, "test", "github.com/test/project")
	gen.StorageType = "ent"

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	type DeviceSpec struct {
		Name string `json:"name"`
	}

	type Device struct {
		Spec DeviceSpec
	}

	if err := gen.RegisterResource(&Device{}); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}

	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas failed: %v", err)
	}

	genericSchema := filepath.Join("internal", "storage", "ent", "schema", "resource.go")
	if _, err := os.Stat(genericSchema); os.IsNotExist(err) {
		t.Fatalf("expected generic resource.go to exist")
	}

	deviceSchema := filepath.Join("internal", "storage", "ent", "schema", "device.go")
	if _, err := os.Stat(deviceSchema); !os.IsNotExist(err) {
		t.Error("did not expect device.go to exist (no dedicated annotation)")
	}
}

func TestGenerateEntSchemasMixed(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change to temp directory: %v", err)
	}

	gen := NewGenerator(tmpDir, "test", "github.com/test/project")
	gen.StorageType = "ent"
	gen.DBDriver = "postgres"

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	type TokenSpec struct {
		Value string `json:"value"`
	}

	type Token struct {
		Spec TokenSpec
	}

	type DeviceSpec struct {
		Name string `json:"name"`
	}

	type Device struct {
		Spec DeviceSpec
	}

	sourcePath := filepath.Join(tmpDir, "token_types.go")
	if err := os.WriteFile(sourcePath, []byte(`package fixture

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct { Spec TokenSpec }
type TokenSpec struct {
	// +fabrica:field:sensitive
	Value string `+"`json:\"value\"`"+`
}
`), 0o644); err != nil {
		t.Fatalf("write Token annotation source: %v", err)
	}
	if err := gen.RegisterResourceFromSource(&Token{}, sourcePath); err != nil {
		t.Fatalf("RegisterResource Token failed: %v", err)
	}

	if err := gen.RegisterResource(&Device{}); err != nil {
		t.Fatalf("RegisterResource Device failed: %v", err)
	}

	if err := gen.PrepareResourceAnnotations(); err != nil {
		t.Fatalf("PrepareResourceAnnotations failed: %v", err)
	}

	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas failed: %v", err)
	}

	tokenSchema := filepath.Join("internal", "storage", "ent", "schema", "token.go")
	if _, err := os.Stat(tokenSchema); os.IsNotExist(err) {
		t.Error("expected token.go to exist (has dedicated annotation)")
	}

	deviceSchema := filepath.Join("internal", "storage", "ent", "schema", "device.go")
	if _, err := os.Stat(deviceSchema); !os.IsNotExist(err) {
		t.Error("did not expect device.go to exist (no annotation)")
	}

	genericSchema := filepath.Join("internal", "storage", "ent", "schema", "resource.go")
	if _, err := os.Stat(genericSchema); os.IsNotExist(err) {
		t.Error("expected generic resource.go to exist (for Device)")
	}
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestGenerateEntSchemasDedicated(t *testing.T) {
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
		Value       string `json:"value"`
		DisplayName string `json:"display_name"`
	}

	type Token struct {
		Spec TokenSpec
	}

	if err := gen.RegisterResource(&Token{}); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}

	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	valueAnnots := annotations.NewFieldAnnotations("Value")
	valueAnnots.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{
			Algorithm: annotations.HashAlgorithmBcrypt,
			Cost:      12,
		},
	}
	valueAnnots.Sensitive = true
	valueAnnots.Immutable = true
	annots.Fields["Value"] = valueAnnots

	displayNameAnnots := annotations.NewFieldAnnotations("DisplayName")
	displayNameAnnots.Index = &annotations.IndexConfig{
		Type: annotations.IndexTypeBTree,
	}
	displayNameAnnots.Unique = true
	annots.Fields["DisplayName"] = displayNameAnnots

	gen.SetResourceAnnotations("Token", annots)

	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas failed: %v", err)
	}

	schemaFile := filepath.Join("internal", "storage", "ent", "schema", "token.go")
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		t.Fatalf("expected dedicated schema file %s to exist", schemaFile)
	}

	content, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("read schema file: %v", err)
	}

	schemaStr := string(content)

	if !strings.Contains(schemaStr, "type Token struct") {
		t.Error("expected Token struct definition")
	}

	if !strings.Contains(schemaStr, "bcrypt") {
		t.Error("expected bcrypt hash reference for Value field")
	}

	if !strings.Contains(schemaStr, "Sensitive()") {
		t.Error("expected Sensitive() for Value field")
	}

	if !strings.Contains(schemaStr, "Immutable()") {
		t.Error("expected Immutable() for Value field")
	}

	if !strings.Contains(schemaStr, "Unique()") {
		t.Error("expected Unique() for DisplayName field")
	}

	if !strings.Contains(schemaStr, "index.Fields(\"display_name\")") {
		t.Error("expected index on DisplayName field")
	}
}

func TestGenerateEntSchemasRejectsReservedSpecColumn(t *testing.T) {
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
		Name string `json:"name"`
	}

	type Token struct {
		Spec TokenSpec
	}

	if err := gen.RegisterResource(&Token{}); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}

	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated
	annots.Fields["Name"] = annotations.NewFieldAnnotations("Name")
	gen.SetResourceAnnotations("Token", annots)

	err = gen.GenerateEntSchemas()
	if err == nil || !strings.Contains(err.Error(), "collides with a dedicated Ent metadata column") {
		t.Fatalf("expected reserved column collision, got %v", err)
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
		DisplayName string `json:"display_name"`
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
		DisplayName string `json:"display_name"`
	}

	type Device struct {
		Spec DeviceSpec
	}

	if err := gen.RegisterResource(&Token{}); err != nil {
		t.Fatalf("RegisterResource Token failed: %v", err)
	}

	if err := gen.RegisterResource(&Device{}); err != nil {
		t.Fatalf("RegisterResource Device failed: %v", err)
	}

	tokenAnnots := annotations.NewResourceAnnotations()
	tokenAnnots.IsResource = true
	tokenAnnots.StorageMode = annotations.StorageModeDedicated
	tokenAnnots.Fields["Value"] = annotations.NewFieldAnnotations("Value")
	gen.SetResourceAnnotations("Token", tokenAnnots)

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

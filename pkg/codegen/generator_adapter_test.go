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

func TestGenerateEntAdapterDedicated(t *testing.T) {
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
		Name  string `json:"name"`
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

	nameAnnots := annotations.NewFieldAnnotations("Name")
	nameAnnots.Index = &annotations.IndexConfig{
		Type: annotations.IndexTypeBTree,
	}
	annots.Fields["Name"] = nameAnnots

	gen.SetResourceAnnotations("Token", annots)

	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("GenerateEntAdapter failed: %v", err)
	}

	genericAdapter := filepath.Join("internal", "storage", "ent_adapter.go")
	if _, err := os.Stat(genericAdapter); os.IsNotExist(err) {
		t.Fatal("expected generic ent_adapter.go to exist")
	}

	dedicatedAdapter := filepath.Join("internal", "storage", "ent_adapter_token.go")
	if _, err := os.Stat(dedicatedAdapter); os.IsNotExist(err) {
		t.Fatalf("expected dedicated adapter %s to exist", dedicatedAdapter)
	}

	content, err := os.ReadFile(dedicatedAdapter)
	if err != nil {
		t.Fatalf("read adapter file: %v", err)
	}

	adapterStr := string(content)

	if !strings.Contains(adapterStr, "ToEntToken") {
		t.Error("expected ToEntToken function")
	}

	if !strings.Contains(adapterStr, "FromEntToken") {
		t.Error("expected FromEntToken function")
	}

	if !strings.Contains(adapterStr, "UpdateTokenFromResource") {
		t.Error("expected UpdateTokenFromResource function")
	}

	if !strings.Contains(adapterStr, "QueryTokenByName") {
		t.Error("expected QueryTokenByName function")
	}

	updateBody := generatedSection(t, adapterStr, "func UpdateTokenFromResource", "func QueryTokenByName")
	if strings.Contains(updateBody, "SetSpecValue(resource.Spec.Value)") {
		t.Error("immutable Value field must not have an update setter")
	}
}

func TestGenerateEntAdapterGenericOnly(t *testing.T) {
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

	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("GenerateEntAdapter failed: %v", err)
	}

	genericAdapter := filepath.Join("internal", "storage", "ent_adapter.go")
	if _, err := os.Stat(genericAdapter); os.IsNotExist(err) {
		t.Fatal("expected generic ent_adapter.go to exist")
	}

	dedicatedAdapter := filepath.Join("internal", "storage", "ent_adapter_device.go")
	if _, err := os.Stat(dedicatedAdapter); !os.IsNotExist(err) {
		t.Error("did not expect dedicated adapter for Device (no annotation)")
	}
}

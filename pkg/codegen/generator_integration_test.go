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

// TestIntegrationTokenService is an end-to-end test of the annotation pipeline
// using a realistic Token service example.
//
// Pipeline tested:
//  1. Parse annotations from token_types.go
//  2. Register resource
//  3. Attach annotations
//  4. Generate dedicated Ent schema
//  5. Generate dedicated storage adapter
//
// This test validates that all phases (1-4) work together correctly.
func TestIntegrationTokenService(t *testing.T) {
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

	// Step 1: Copy example token service
	// Find project root (go up from pkg/codegen to project root)
	projectRoot := filepath.Join(origDir, "..", "..")
	examplePath := filepath.Join(projectRoot, "examples", "token-service", "apis", "v1", "token_types.go")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Skipf("example token service not found at %s - skipping integration test", examplePath)
	}

	apisDir := filepath.Join("apis", "v1")
	if err := os.MkdirAll(apisDir, 0755); err != nil {
		t.Fatalf("create apis directory: %v", err)
	}

	tokenTypesContent, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read example token_types.go: %v", err)
	}

	tokenTypesPath := filepath.Join(apisDir, "token_types.go")
	if err := os.WriteFile(tokenTypesPath, tokenTypesContent, 0644); err != nil {
		t.Fatalf("write token_types.go: %v", err)
	}

	// Step 2: Initialize generator
	gen := NewGenerator(tmpDir, "tokenservice", "github.com/test/token-service")
	gen.StorageType = "ent"
	gen.DBDriver = "postgres"

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	// Step 3: Parse annotations from the actual source file
	annots, err := gen.ParseResourceAnnotations(tokenTypesPath, "Token")
	if err != nil {
		t.Fatalf("ParseResourceAnnotations failed: %v", err)
	}

	// Validate parsed annotations
	if !annots.IsResource {
		t.Error("expected IsResource=true")
	}

	if annots.StorageMode != "dedicated" {
		t.Errorf("expected storage=dedicated, got %s", annots.StorageMode)
	}

	if len(annots.Fields) == 0 {
		t.Fatal("expected field annotations to be parsed")
	}

	// Validate Value field annotations (bcrypt hashing)
	valueField, ok := annots.Fields["Value"]
	if !ok {
		t.Fatal("expected Value field annotations")
	}

	if valueField.Storage == nil || valueField.Storage.Type != "hashed" {
		t.Error("expected Value field to have hashed storage")
	}

	if valueField.Storage.Hash.Algorithm != "bcrypt" {
		t.Errorf("expected bcrypt algorithm, got %s", valueField.Storage.Hash.Algorithm)
	}

	if valueField.Storage.Hash.Cost != 12 {
		t.Errorf("expected bcrypt cost=12, got %d", valueField.Storage.Hash.Cost)
	}

	if !valueField.Sensitive {
		t.Error("expected Value field to be sensitive")
	}

	if !valueField.Immutable {
		t.Error("expected Value field to be immutable")
	}

	// Validate Name field annotations (unique + index)
	nameField, ok := annots.Fields["Name"]
	if !ok {
		t.Fatal("expected Name field annotations")
	}

	if !nameField.Unique {
		t.Error("expected Name field to be unique")
	}

	if nameField.Index == nil {
		t.Error("expected Name field to have index")
	}

	// Validate Revoked field annotations (default value)
	revokedField, ok := annots.Fields["Revoked"]
	if !ok {
		t.Fatal("expected Revoked field annotations")
	}

	if revokedField.Default != "false" {
		t.Errorf("expected Revoked default=false, got %s", revokedField.Default)
	}

	// Step 4: Register resource (simulated - we can't import the actual type)
	// In real usage, CLI would do: gen.RegisterResource(&v1.Token{})
	// For testing, we'll manually create metadata
	metadata := ResourceMetadata{
		Name:         "Token",
		PluralName:   "tokens",
		Package:      "github.com/test/token-service/apis/v1",
		PackageAlias: "v1",
		TypeName:     "*v1.Token",
		SpecType:     "v1.TokenSpec",
		StatusType:   "v1.TokenStatus",
		URLPath:      "/tokens",
		StorageName:  "Token",
		Tags:         make(map[string]string),
		SpecFields: []SpecField{
			{Name: "Value", JSONName: "value", Type: "string", Required: true},
			{Name: "Name", JSONName: "name", Type: "string", Required: true},
			{Name: "Description", JSONName: "description", Type: "string", Required: false},
			{Name: "ExpiresAt", JSONName: "expiresAt", Type: "int64", Required: false},
			{Name: "Revoked", JSONName: "revoked", Type: "bool", Required: false},
			{Name: "Scopes", JSONName: "scopes", Type: "[]string", Required: false},
		},
		Annotations: annots,
	}

	gen.Resources = append(gen.Resources, metadata)

	// Step 5: Generate Ent schema
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas failed: %v", err)
	}

	// Verify dedicated schema was generated
	tokenSchemaPath := filepath.Join("internal", "storage", "ent", "schema", "token.go")
	if _, err := os.Stat(tokenSchemaPath); os.IsNotExist(err) {
		t.Fatal("expected token.go schema to be generated")
	}

	schemaContent, err := os.ReadFile(tokenSchemaPath)
	if err != nil {
		t.Fatalf("read token schema: %v", err)
	}

	schemaStr := string(schemaContent)

	// Validate schema contains expected elements
	checks := []struct {
		pattern string
		desc    string
	}{
		{"type Token struct", "Token struct definition"},
		{"func (Token) Fields()", "Fields method"},
		{"func (Token) Indexes()", "Indexes method"},
		{"bcrypt", "bcrypt reference for Value field"},
		{"Sensitive()", "Sensitive() for Value field"},
		{"Immutable()", "Immutable() for Value field"},
		{"Unique()", "Unique() for Name field"},
		{"index.Fields(\"name\")", "index on Name field"},
		{"Default(false)", "Default(false) for Revoked field"},
	}

	for _, check := range checks {
		if !strings.Contains(schemaStr, check.pattern) {
			t.Errorf("schema missing %s (pattern: %q)", check.desc, check.pattern)
		}
	}

	// Step 6: Generate storage adapter
	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("GenerateEntAdapter failed: %v", err)
	}

	// Verify dedicated adapter was generated
	tokenAdapterPath := filepath.Join("internal", "storage", "ent_adapter_token.go")
	if _, err := os.Stat(tokenAdapterPath); os.IsNotExist(err) {
		t.Fatal("expected ent_adapter_token.go to be generated")
	}

	adapterContent, err := os.ReadFile(tokenAdapterPath)
	if err != nil {
		t.Fatalf("read token adapter: %v", err)
	}

	adapterStr := string(adapterContent)

	// Validate adapter contains expected functions
	adapterChecks := []struct {
		pattern string
		desc    string
	}{
		{"func ToEntToken", "ToEntToken function"},
		{"func FromEntToken", "FromEntToken function"},
		{"func UpdateTokenFromResource", "UpdateTokenFromResource function"},
		{"func QueryTokenByName", "QueryTokenByName function"},
		{"func ListTokens", "ListTokens function"},
		{"func DeleteTokenByUID", "DeleteTokenByUID function"},
		{"is immutable", "immutability comment for Value field"},
	}

	for _, check := range adapterChecks {
		if !strings.Contains(adapterStr, check.pattern) {
			t.Errorf("adapter missing %s (pattern: %q)", check.desc, check.pattern)
		}
	}

	t.Logf("✅ Integration test passed - complete pipeline validated")
	t.Logf("   Generated schema: %s", tokenSchemaPath)
	t.Logf("   Generated adapter: %s", tokenAdapterPath)
}

//go:build integration

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	resourcepkg "github.com/openchami/fabrica/pkg/resource"
)

type LegacyToken struct {
	resourcepkg.Resource
	Spec   LegacyTokenSpec
	Status LegacyTokenStatus
}

type LegacyTokenSpec struct {
	DisplayName string `json:"display-name" validate:"required"`
	Metadata    string `json:"metadata" validate:"required"`
}

type LegacyTokenStatus struct {
	State string
}

func TestGeneratedDedicatedAdapter_non_versioned_shape_builds(t *testing.T) {
	// Given
	root := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change to legacy fixture: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join(originalDir, "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	sourcePath := filepath.Join(root, "pkg", "resources", "token", "token.go")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("create legacy resource directory: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte(legacyDedicatedTokenSource), 0o644); err != nil {
		t.Fatalf("write legacy resource: %v", err)
	}
	goMod := strings.ReplaceAll(legacyDedicatedGoMod, "REPO_ROOT", filepath.ToSlash(repoRoot))
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write legacy go.mod: %v", err)
	}

	gen := NewGenerator(root, "legacy", "example.com/legacy-envelope")
	gen.StorageType = "ent"
	gen.DBDriver = "sqlite"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	if err := gen.RegisterResourceFromSource(&LegacyToken{}, sourcePath); err != nil {
		t.Fatalf("register legacy resource: %v", err)
	}
	gen.Resources[0].Package = "example.com/legacy-envelope/pkg/resources/token"
	gen.Resources[0].PackageAlias = "token"
	if err := gen.PrepareResourceAnnotations(); err != nil {
		t.Fatalf("prepare legacy annotations: %v", err)
	}
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("generate legacy schemas: %v", err)
	}
	runDedicatedSchemaCommand(t, root, "go", "mod", "tidy")
	runDedicatedSchemaCommand(t, root, "go", "run", "entgo.io/ent/cmd/ent@v0.14.5", "generate", "./internal/storage/ent/schema")
	if err := gen.GenerateStorage(); err != nil {
		t.Fatalf("generate legacy storage: %v", err)
	}
	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("generate legacy adapters: %v", err)
	}
	adapter, err := os.ReadFile(filepath.Join(root, "internal", "storage", "ent_adapter_legacytoken.go"))
	if err != nil {
		t.Fatalf("read legacy adapter: %v", err)
	}
	if !strings.Contains(string(adapter), "Resource: resource.Resource{") {
		t.Fatalf("non-versioned adapter did not reconstruct embedded envelope\n%s", adapter)
	}
	runDedicatedSchemaCommand(t, root, "go", "mod", "tidy")

	// When
	runDedicatedSchemaCommand(t, root, "go", "build", "./...")

	// Then
	if _, err := os.Stat(filepath.Join(root, "internal", "storage", "ent", "legacytoken.go")); err != nil {
		t.Fatalf("legacy Ent entity: %v", err)
	}
}

const legacyDedicatedGoMod = `module example.com/legacy-envelope

go 1.24.0

require (
	entgo.io/ent v0.14.5
	github.com/openchami/fabrica v0.0.0
)

replace github.com/openchami/fabrica => REPO_ROOT
`

const legacyDedicatedTokenSource = `package token

import "github.com/openchami/fabrica/pkg/resource"

// +fabrica:resource
// +fabrica:storage=dedicated
type LegacyToken struct {
	resource.Resource
	Spec LegacyTokenSpec ` + "`json:\"spec\"`" + `
	Status LegacyTokenStatus ` + "`json:\"status,omitempty\"`" + `
}

type LegacyTokenSpec struct {
	// +fabrica:field:index
	DisplayName string ` + "`json:\"display-name\" validate:\"required\"`" + `
	Metadata string ` + "`json:\"metadata\" validate:\"required\"`" + `
}

type LegacyTokenStatus struct { State string ` + "`json:\"state\"`" + ` }
`

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedLegacyHandlers_compile_for_file_and_ent_storage(t *testing.T) {
	for _, storageType := range []string{"file", "ent"} {
		t.Run(storageType, func(t *testing.T) {
			// Given
			project := newGeneratedProject(t, storageType)
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("get working directory: %v", err)
			}
			if err := os.Chdir(project.root); err != nil {
				t.Fatalf("change to legacy project: %v", err)
			}
			t.Cleanup(func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("restore working directory: %v", err)
				}
			})
			if err := os.Remove(filepath.Join(project.root, "apis.yaml")); err != nil {
				t.Fatalf("remove APIs layout config: %v", err)
			}
			if err := os.RemoveAll(filepath.Join(project.root, "apis")); err != nil {
				t.Fatalf("remove APIs layout resources: %v", err)
			}
			resourcePath := filepath.Join(project.root, "pkg", "resources", "token", "token.go")
			if err := os.MkdirAll(filepath.Dir(resourcePath), 0o755); err != nil {
				t.Fatalf("create legacy resource directory: %v", err)
			}
			if err := os.WriteFile(resourcePath, []byte(legacyHandlerTokenSource), 0o644); err != nil {
				t.Fatalf("write legacy resource: %v", err)
			}
			mainPath := filepath.Join(project.root, "cmd", "server", "main.go")
			if err := os.WriteFile(mainPath, []byte("package main\n\ntype Config struct{}\n\nfunc main() {}\n"), 0o644); err != nil {
				t.Fatalf("write server fixture: %v", err)
			}
			gen := NewGenerator(filepath.Join(project.root, "cmd", "server"), "main", "example.com/generated-annotation-acceptance")
			gen.SetStorageType(storageType)
			gen.Config.StorageType = storageType
			gen.SetDBDriver("sqlite")
			gen.Config.DBDriver = "sqlite"
			if err := gen.RegisterResourceFromSource(&LegacyToken{}, resourcePath); err != nil {
				t.Fatalf("register legacy resource: %v", err)
			}
			gen.Resources[0].Package = "example.com/generated-annotation-acceptance/pkg/resources/token"
			gen.Resources[0].PackageAlias = "token"
			gen.Resources[0].TypeName = "*token.LegacyToken"
			gen.Resources[0].SpecType = "token.LegacyTokenSpec"
			gen.Resources[0].StatusType = "token.LegacyTokenStatus"

			// When
			if storageType == "ent" {
				if err := gen.PrepareResourceAnnotations(); err != nil {
					t.Fatalf("prepare legacy annotations: %v", err)
				}
				if err := gen.LoadTemplates(); err != nil {
					t.Fatalf("load templates: %v", err)
				}
				if err := gen.GenerateEntSchemas(); err != nil {
					t.Fatalf("generate legacy Ent schemas: %v", err)
				}
				if result := project.tidy(t); result.err != nil {
					t.Fatalf("%s", result.failureMessage())
				}
				if result := project.run(t, "legacy-ent-codegen", project.root, "go", "run", "entgo.io/ent/cmd/ent@v0.14.5", "generate", "./internal/storage/ent/schema"); result.err != nil {
					t.Fatalf("%s", result.failureMessage())
				}
			}
			generateErr := gen.GenerateAll()

			// Then
			if generateErr != nil {
				t.Fatalf("generate complete legacy project: %v", generateErr)
			}
			if result := project.tidy(t); result.err != nil {
				t.Fatalf("%s", result.failureMessage())
			}
			if result := project.build(t); result.err != nil {
				t.Fatalf("%s", result.failureMessage())
			}
		})
	}
}

const legacyHandlerTokenSource = `package token

import "github.com/openchami/fabrica/pkg/resource"

type LegacyToken struct {
	resource.Resource
	Spec LegacyTokenSpec ` + "`json:\"spec\"`" + `
	Status LegacyTokenStatus ` + "`json:\"status,omitempty\"`" + `
}

type LegacyTokenSpec struct {
	DisplayName string ` + "`json:\"displayName\"`" + `
	Metadata string ` + "`json:\"metadata\"`" + `
}
type LegacyTokenStatus struct { State string ` + "`json:\"state\"`" + ` }
`

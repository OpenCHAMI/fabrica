// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Compile the actual templates and compare the served document with HTTP
// behavior, including regeneration between policies in the same service.
func TestGeneratedOpenAPIRequiredFields(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("testdata/openapi_required_test.go.fixture")
	if err != nil {
		t.Fatal(err)
	}
	validationTypes, err := os.ReadFile("testdata/openapi_validation_types.go.fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, revisioning := range []bool{false, true} {
		t.Run(fmt.Sprintf("revisioning=%t", revisioning), func(t *testing.T) {
			dir := t.TempDir()
			writeRevisionFixture(t, dir, root)
			if err := os.Remove(filepath.Join(dir, "revision_workflow_test.go")); err != nil {
				t.Fatal(err)
			}
			typesPath := filepath.Join(dir, "apis/v1/types.go")
			types, err := os.ReadFile(typesPath)
			if err != nil {
				t.Fatal(err)
			}
			types = bytes.ReplaceAll(types, []byte(`Status NodeStatus `+"`"+`json:"status"`+"`"), []byte(`Status NodeStatus `+"`"+`json:"status" validate:"omitempty"`+"`"))
			for _, name := range []string{"NodeSpec", "NodeStatus"} {
				prefix := []byte("type " + name + " struct {")
				types = bytes.Replace(types, prefix, append(prefix, []byte(" OpenAPIValidationFields;")...), 1)
			}
			if revisioning {
				types = bytes.ReplaceAll(types, []byte(`validate:"omitempty,oneof=Pending Ready"`), []byte(`validate:"required,oneof=Pending Ready"`))
			}
			if err := os.WriteFile(typesPath, types, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "apis/v1/openapi_validation_types.go"), validationTypes, 0644); err != nil {
				t.Fatal(err)
			}
			gen := NewGenerator(dir, "main", "example.com/revisionfixture")
			gen.Config.VersioningEnabled = true
			gen.Config.ConditionalEnabled = false
			gen.Resources = []ResourceMetadata{{Name: "Node", PluralName: "nodes", Package: "example.com/revisionfixture/apis/v1", PackageAlias: "v1", TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", URLPath: "/nodes", StorageName: "Node", RevisioningEnabled: revisioning, BareNameSelector: "default"}}
			if err := gen.LoadTemplates(); err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"strict", "warn", "disabled", "strict"} {
				gen.Config.ValidationMode = mode
				gen.Config.ValidationEnabled = true
				for _, generate := range []func() error{gen.GenerateModels, gen.GenerateHandlers, gen.GenerateRoutes, gen.GenerateOpenAPI} {
					if err := generate(); err != nil {
						t.Fatal(err)
					}
				}
				var storage bytes.Buffer
				if err := gen.Templates["storage"].Execute(&storage, gen.globalTemplateData("storage/file.go.tmpl")); err != nil {
					t.Fatal(err)
				}
				source, err := format.Source(storage.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(dir, "internal/storage"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "internal/storage/storage_generated.go"), source, 0644); err != nil {
					t.Fatal(err)
				}
				test := strings.ReplaceAll(string(fixture), "EXPECTED_MODE", mode)
				test = strings.ReplaceAll(test, "REVISIONING", fmt.Sprint(revisioning))
				if err := os.WriteFile(filepath.Join(dir, "openapi_required_test.go"), []byte(test), 0644); err != nil {
					t.Fatal(err)
				}
				for _, args := range [][]string{{"mod", "tidy"}, {"test", "-count=1", "./..."}} {
					cmd := exec.Command("go", args...)
					cmd.Dir = dir
					output, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("%s go %v: %v\n%s", mode, args, err, output)
					}
				}
			}
		})
	}
}

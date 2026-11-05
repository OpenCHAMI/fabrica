// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestLoadResourceAnnotations(t *testing.T) {
	for _, tc := range []struct{ name, annotation, backend, wantErr string }{
		{"dedicated", "// +fabrica:resource\n// +fabrica:storage=dedicated", "ent", ""},
		{"generic", "", "ent", ""},
		{"invalid", "// +fabrica:storage=typo", "ent", "unknown storage mode"},
		{"wrong backend", "// +fabrica:storage=dedicated", "file", "requires the ent backend"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for _, version := range []string{"v1", "v2"} {
				dir := filepath.Join("apis", version)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				// Neither source file has the conventional *_types.go name.
				resourceSource := "package " + version + "\n" + tc.annotation + "\ntype Token struct { Spec TokenSpec }\n"
				specSource := "package " + version + "\ntype TokenSpec struct {\n// +fabrica:field:unique\nValue string `json:\"value\"`\n}\n"
				if tc.name == "generic" {
					specSource = strings.ReplaceAll(specSource, "// +fabrica:field:unique\n", "")
				}
				for name, body := range map[string]string{"envelope.go": resourceSource, "spec.go": specSource} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			g := NewGenerator(".", "test", "example.com/service")
			g.StorageType = tc.backend
			g.DBDriver = "sqlite"
			for _, version := range []string{"v1", "v2"} {
				g.Resources = append(g.Resources, ResourceMetadata{Name: "Token", Package: "example.com/service/apis/" + version, SpecFields: []SpecField{{Name: "Value", JSONName: "value", Type: "string"}}})
			}
			err := g.LoadResourceAnnotations()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error=%v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range g.Resources {
				if tc.name == "generic" {
					if r.Annotations != nil {
						t.Fatal("unannotated resource changed")
					}
					continue
				}
				if r.Annotations == nil || r.Annotations.StorageMode != annotations.StorageModeDedicated || !r.Annotations.Fields["Value"].Unique {
					t.Fatalf("annotations not attached: %+v", r.Annotations)
				}
			}
		})
	}
}

func TestLoadResourceAnnotationsMissingSource(t *testing.T) {
	t.Chdir(t.TempDir())
	g := NewGenerator(".", "test", "example.com/service")
	g.Resources = []ResourceMetadata{{Name: "Token", Package: "example.com/service/missing"}}
	if err := g.LoadResourceAnnotations(); err == nil {
		t.Fatal("missing source silently accepted")
	}
}

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMixedStorage_generated_runtime_routes_each_resource_exclusively(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, mixedDedicatedTokenSource)
	widgetPath := filepath.Join(project.root, "apis", "acceptance.example.io", "v1", "widget_types.go")
	if err := os.WriteFile(widgetPath, []byte(mixedGenericWidgetSource), 0o644); err != nil {
		t.Fatalf("write generic resource: %v", err)
	}
	apisPath := filepath.Join(project.root, "apis.yaml")
	apis := "groups:\n  - name: acceptance.example.io\n    storageVersion: v1\n    versions: [v1]\n    resources: [Token, Widget]\n"
	if err := os.WriteFile(apisPath, []byte(apis), 0o644); err != nil {
		t.Fatalf("write mixed API config: %v", err)
	}
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.run(t, "mixed-ent-codegen", project.root, "go", "generate", "./internal/storage"); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	testPath := filepath.Join(project.root, "internal", "storage", "mixed_storage_runtime_test.go")
	if err := os.WriteFile(testPath, []byte(mixedStorageRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated mixed runtime test: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	// When
	result := project.run(t, "mixed-storage-runtime", project.root, "go", "test", "-count=1", "-v", "-run", "TestMixedStorage", "./internal/storage")

	// Then
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	for _, receipt := range []string{"exclusive create update delete", "dedicated query helpers", "conversion failure no partial list"} {
		if !strings.Contains(result.stdout, receipt) {
			t.Errorf("runtime output missing %q receipt\n%s", receipt, result.stdout)
		}
	}
}

const mixedDedicatedTokenSource = `package v1

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       TokenSpec        ` + "`json:\"spec\"`" + `
	Status     TokenStatus      ` + "`json:\"status,omitempty\"`" + `
}

type TokenSpec struct {
	// +fabrica:field:index
	DisplayName string ` + "`json:\"displayName\"`" + `
}

type TokenStatus struct {
	State string ` + "`json:\"state\"`" + `
}

func (s *TokenStatus) UnmarshalJSON(data []byte) error {
	type tokenStatus TokenStatus
	var decoded tokenStatus
	if err := json.Unmarshal(data, &decoded); err != nil { return err }
	if decoded.State == "corrupt" { return errors.New("corrupt token status") }
	*s = TokenStatus(decoded)
	return nil
}

func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string                { return "Token" }
func (r *Token) GetName() string                { return r.Metadata.Name }
func (r *Token) GetUID() string                 { return r.Metadata.UID }
func (r *Token) IsHub()                         {}
`

const mixedGenericWidgetSource = `package v1

import (
	"context"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
type Widget struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       WidgetSpec       ` + "`json:\"spec\"`" + `
	Status     WidgetStatus     ` + "`json:\"status,omitempty\"`" + `
}

type WidgetSpec struct { Value string ` + "`json:\"value\"`" + ` }
type WidgetStatus struct { State string ` + "`json:\"state\"`" + ` }

func (r *Widget) Validate(context.Context) error { return nil }
func (r *Widget) GetKind() string                { return "Widget" }
func (r *Widget) GetName() string                { return r.Metadata.Name }
func (r *Widget) GetUID() string                 { return r.Metadata.UID }
func (r *Widget) IsHub()                         {}
`

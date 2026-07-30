// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedAnnotationProject_generic_storage_CRUD_and_queries_remain_compatible(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, genericStorageTokenSource)
	generateResult := project.generate(t)
	if generateResult.err != nil {
		t.Fatalf("%s", generateResult.failureMessage())
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.run(t, "generic-ent-codegen", project.root, "go", "generate", "./internal/storage"); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	testPath := filepath.Join(project.root, "internal", "storage", "generic_storage_runtime_test.go")
	if err := os.WriteFile(testPath, []byte(genericStorageRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated generic runtime test: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	// When
	result := project.run(t, "generic-storage-runtime", project.root, "go", "test", "-count=1", "-v", "-run", "TestGenericStorage", "./internal/storage")

	// Then
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	for _, receipt := range []string{"generic CRUD", "generic query helpers", "generic not found", "generic metadata zero and changed", "generic create conflict chain", "generic update conflict chain"} {
		if !strings.Contains(result.stdout, receipt) {
			t.Errorf("runtime output missing %q receipt\n%s", receipt, result.stdout)
		}
	}
}

const genericStorageTokenSource = `package v1

import (
	"context"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
type Token struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       TokenSpec        ` + "`json:\"spec\"`" + `
	Status     TokenStatus      ` + "`json:\"status,omitempty\"`" + `
}

type TokenSpec struct {
	Value string ` + "`json:\"value\"`" + `
}

type TokenStatus struct {
	State string ` + "`json:\"state\"`" + `
}

func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string                { return "Token" }
func (r *Token) GetName() string                { return r.Metadata.Name }
func (r *Token) GetUID() string                 { return r.Metadata.UID }
func (r *Token) IsHub()                         {}
`

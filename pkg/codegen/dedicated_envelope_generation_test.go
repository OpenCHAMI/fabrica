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

func TestDedicatedEnvelope_schema_renders_complete_envelope(t *testing.T) {
	// Given
	schema := generateDedicatedShapeSchema(t, "sqlite").content

	// When
	wantFragments := []string{
		`field.JSON("status", json.RawMessage{})`,
		`field.JSON("labels", map[string]string{})`,
		`field.JSON("annotations", map[string]string{})`,
		`field.String("resource_version")`,
		`field.Time("created_at")`,
		`field.Time("updated_at")`,
	}

	// Then
	for _, fragment := range wantFragments {
		if !strings.Contains(schema, fragment) {
			t.Errorf("dedicated schema missing envelope field %q\n%s", fragment, schema)
		}
	}
}

func TestGeneratedDedicatedAdapter_uses_explicit_spec_and_envelope_mappings(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, dedicatedEnvelopeTokenSource)
	result := project.generate(t)
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	adapterPath := filepath.Join(project.root, "internal", "storage", "ent_adapter_token.go")
	adapterBytes, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("read generated dedicated adapter: %v", err)
	}

	// When
	adapter := string(adapterBytes)
	wantFragments := []string{
		"SetSpecDisplayName(resource.Spec.DisplayName)",
		"SetSpecMetadata(resource.Spec.Metadata)",
		"SetLabels(resource.Metadata.Labels)",
		"SetAnnotations(resource.Metadata.Annotations)",
		"json.Marshal(resource.Status)",
		"json.Unmarshal(entity.Status, &status)",
		"Status: status",
		"DisplayName: entity.SpecDisplayName",
		"Metadata: entity.SpecMetadata",
	}

	// Then
	for _, fragment := range wantFragments {
		if !strings.Contains(adapter, fragment) {
			t.Errorf("generated dedicated adapter missing %q\n%s", fragment, adapter)
		}
	}
	for _, stale := range []string{
		"SetDisplayName(resource.Spec.DisplayName)",
		"SetMetadata(resource.Spec.Metadata)",
		"DisplayName: entity.DisplayName",
		"Metadata: entity.Metadata",
	} {
		if strings.Contains(adapter, stale) {
			t.Errorf("generated dedicated adapter contains stale inferred mapping %q\n%s", stale, adapter)
		}
	}
}

const dedicatedEnvelopeTokenSource = `package v1

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
	DisplayName string ` + "`json:\"display-name\" validate:\"required\"`" + `
	Metadata string ` + "`json:\"metadata\" validate:\"required\"`" + `
}

type TokenStatus struct {
	State string ` + "`json:\"state\"`" + `
	FailMarshal bool ` + "`json:\"-\"`" + `
}

func (s TokenStatus) MarshalJSON() ([]byte, error) {
	if s.FailMarshal { return nil, errors.New("forced status marshal failure") }
	type statusAlias TokenStatus
	return json.Marshal(statusAlias(s))
}

func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string                { return "Token" }
func (r *Token) GetName() string                { return r.Metadata.Name }
func (r *Token) GetUID() string                 { return r.Metadata.UID }
func (r *Token) IsHub()                         {}
`

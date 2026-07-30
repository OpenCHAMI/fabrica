// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"strings"
	"testing"
)

func TestGeneratedHandlers_map_create_and_update_storage_conflicts_to_stable_409(t *testing.T) {
	// Given
	gen := NewGenerator(t.TempDir(), "main", "example.com/conflict")
	gen.Version = "test"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	resource := ResourceMetadata{
		Name:         "Token",
		PluralName:   "tokens",
		Package:      "example.com/conflict/apis/example.io/v1",
		PackageAlias: "v1",
		TypeName:     "*v1.Token",
		SpecType:     "v1.TokenSpec",
		StatusType:   "v1.TokenStatus",
		URLPath:      "/tokens",
		StorageName:  "Token",
	}
	var output bytes.Buffer

	// When
	if err := gen.Templates["handlers"].Execute(&output, gen.templateData(resource, "server/handlers.go.tmpl")); err != nil {
		t.Fatal(err)
	}

	// Then
	generated := output.String()
	if got := strings.Count(generated, "errors.Is(err, storage.ErrStorageConflict)"); got != 3 {
		t.Fatalf("generated conflict checks=%d, want 3\n%s", got, generated)
	}
	if got := strings.Count(generated, "respondError(w, http.StatusConflict, storage.ErrStorageConflict)"); got != 3 {
		t.Fatalf("generated stable 409 mappings=%d, want 3\n%s", got, generated)
	}
}

func TestGeneratedStorageErrors_define_backend_independent_conflict_contract(t *testing.T) {
	// Given
	gen := NewGenerator(t.TempDir(), "main", "example.com/conflict")
	gen.Version = "test"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer

	// When
	tmpl, ok := gen.Templates["storageErrors"]
	if !ok {
		t.Fatal("shared storage error template is not registered")
	}
	if err := tmpl.Execute(&output, gen.globalTemplateData("storage/errors.go.tmpl")); err != nil {
		t.Fatal(err)
	}

	// Then
	generated := output.String()
	for _, contract := range []string{
		`var ErrStorageConflict = errors.New("storage conflict")`,
		`type StorageConflictError struct`,
		`func (e *StorageConflictError) Unwrap() error`,
		`func (e *StorageConflictError) Is(target error) bool`,
	} {
		if !strings.Contains(generated, contract) {
			t.Errorf("generated shared storage errors missing %q\n%s", contract, generated)
		}
	}
	if strings.Contains(generated, "entgo.io/ent") {
		t.Fatalf("shared storage errors import Ent\n%s", generated)
	}
}

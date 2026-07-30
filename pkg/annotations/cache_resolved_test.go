// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const resolvedAnnotationSource = `package test

// +fabrica:resource
// +fabrica:storage=dedicated
type Widget struct { Spec WidgetSpec }

type WidgetSpec struct {
	// +fabrica:field:default=0
	Count int ` + "`json:\"count\"`" + `

	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	Secret string ` + "`json:\"secret\"`" + `

	// +fabrica:field:index=gin
	Tags []string ` + "`json:\"tags,omitempty\"`" + `
}
`

func TestCache_resolved_storage_warm_result_deeply_equals_typed_cold_result(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := filepath.Join(t.TempDir(), "types.go")
	if err := os.WriteFile(filename, []byte(resolvedAnnotationSource), 0o644); err != nil {
		t.Fatal(err)
	}

	// When
	cold, err := ResolveStorageIntent(filename, "Widget", DialectPostgreSQL)
	if err != nil {
		t.Fatalf("cold ResolveStorageIntent() error = %v", err)
	}
	warm, err := ResolveStorageIntent(filename, "Widget", DialectPostgreSQL)

	// Then
	if err != nil {
		t.Fatalf("warm ResolveStorageIntent() error = %v", err)
	}
	if globalCache.Size() != 1 {
		t.Fatalf("global cache size = %d, want 1 resolved result", globalCache.Size())
	}
	if !reflect.DeepEqual(warm, cold) {
		t.Fatalf("warm resolved storage differs from cold\ncold: %#v\nwarm: %#v", cold, warm)
	}
	count := resolvedFieldByName(t, warm, "Count")
	defaultValue, ok := count.Default.(IntDefault)
	if !ok || defaultValue.Value != 0 || count.Type.Kind != FieldKindInt {
		t.Fatalf("Count typed IR = %#v", count)
	}
	secret := resolvedFieldByName(t, warm, "Secret")
	if secret.Transform.Kind != TransformBcrypt || secret.Transform.BcryptCost != 12 || !secret.Sensitive {
		t.Fatalf("Secret capability IR = %#v", secret)
	}
	tags := resolvedFieldByName(t, warm, "Tags")
	if tags.Type.Kind != FieldKindStringSlice || tags.Index != IndexGIN || tags.Optionality != OptionalityOptional {
		t.Fatalf("Tags typed IR = %#v", tags)
	}
}

func TestCache_resolved_storage_caller_mutation_does_not_change_cached_IR(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := filepath.Join(t.TempDir(), "types.go")
	if err := os.WriteFile(filename, []byte(resolvedAnnotationSource), 0o644); err != nil {
		t.Fatal(err)
	}
	cold, err := ResolveStorageIntent(filename, "Widget", DialectPostgreSQL)
	if err != nil {
		t.Fatalf("cold ResolveStorageIntent() error = %v", err)
	}

	// When
	cold.Name = "corrupted"
	cold.Source.Directive = "corrupted"
	cold.Fields[0].GoName = "corrupted"
	cold.Fields = append(cold.Fields, ResolvedFieldStorage{GoName: "Injected"})
	warm, err := ResolveStorageIntent(filename, "Widget", DialectPostgreSQL)
	if err != nil {
		t.Fatalf("warm ResolveStorageIntent() error = %v", err)
	}
	warm.Fields[0].Source.FieldName = "mutated"
	later, err := ResolveStorageIntent(filename, "Widget", DialectPostgreSQL)

	// Then
	if err != nil {
		t.Fatalf("later ResolveStorageIntent() error = %v", err)
	}
	if later.Name != "Widget" || len(later.Fields) != 3 {
		t.Fatalf("resolved resource leaked mutation: %#v", later)
	}
	if resolvedFieldByName(t, later, "Count").Source.FieldName != "Count" {
		t.Fatalf("resolved field source leaked mutation: %#v", later.Fields)
	}
}

func resolvedFieldByName(t *testing.T, resource *ResolvedResourceStorage, name string) ResolvedFieldStorage {
	t.Helper()
	for _, field := range resource.Fields {
		if field.GoName == name {
			return field
		}
	}
	t.Fatalf("resolved field %q missing from %#v", name, resource.Fields)
	return ResolvedFieldStorage{}
}

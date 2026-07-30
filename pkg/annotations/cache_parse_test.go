// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

const completeAnnotationSource = `package test

// +fabrica:resource
// +fabrica:storage=dedicated
type Widget struct {
	Spec WidgetSpec
}

type WidgetSpec struct {
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	Secret string ` + "`json:\"secret\"`" + `

	// +fabrica:field:default=guest
	// +fabrica:field:index=btree
	// +fabrica:field:unique
	Name string ` + "`json:\"name\"`" + `
}

type Marker struct{}
`

func writeAnnotationFixture(t *testing.T, source string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "types.go")
	if err := os.WriteFile(filename, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return filename
}

func resetGlobalCache(t *testing.T) {
	t.Helper()
	globalCache.Clear()
	t.Cleanup(globalCache.Clear)
}

func TestParseFileAnnotations_cache_warm_result_deeply_equals_complete_cold_result(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := writeAnnotationFixture(t, completeAnnotationSource)

	// When
	cold, err := ParseFileAnnotations(filename)
	if err != nil {
		t.Fatalf("cold ParseFileAnnotations() error = %v", err)
	}
	warm, err := ParseFileAnnotations(filename)

	// Then
	if err != nil {
		t.Fatalf("warm ParseFileAnnotations() error = %v", err)
	}
	if !reflect.DeepEqual(warm, cold) {
		t.Fatalf("warm parse differs from cold parse\ncold: %#v\nwarm: %#v", cold, warm)
	}
	if !warm["Widget"].IsResource || warm["Widget"].StorageMode != StorageModeDedicated {
		t.Fatalf("warm Widget resource metadata = %#v", warm["Widget"])
	}
	if len(warm["Widget"].RawAnnotations) != 2 {
		t.Fatalf("warm Widget raw annotations = %#v, want 2 directives", warm["Widget"].RawAnnotations)
	}
	secret := warm["WidgetSpec"].Fields["Secret"]
	if secret.Storage == nil || secret.Storage.Hash == nil || secret.Storage.Hash.Cost != 12 || !secret.Sensitive {
		t.Fatalf("warm WidgetSpec.Secret annotations = %#v", secret)
	}
	name := warm["WidgetSpec"].Fields["Name"]
	if name.Default != "guest" || name.Index == nil || name.Index.Type != IndexTypeBTree || !name.Unique {
		t.Fatalf("warm WidgetSpec.Name annotations = %#v", name)
	}
	if _, ok := warm["Marker"]; !ok {
		t.Fatal("warm parse omitted type with no annotated fields")
	}
}

func TestParseFileAnnotations_cache_cold_result_mutation_does_not_change_warm_result(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := writeAnnotationFixture(t, completeAnnotationSource)
	cold, err := ParseFileAnnotations(filename)
	if err != nil {
		t.Fatalf("cold ParseFileAnnotations() error = %v", err)
	}

	// When
	cold["Widget"].IsResource = false
	cold["Widget"].RawAnnotations[0] = "corrupted"
	cold["WidgetSpec"].Fields["Secret"].Storage.Hash.Cost = 4
	cold["WidgetSpec"].Fields["Name"].RawAnnotations[0] = "corrupted"
	delete(cold, "Marker")
	warm, err := ParseFileAnnotations(filename)

	// Then
	if err != nil {
		t.Fatalf("warm ParseFileAnnotations() error = %v", err)
	}
	assertCompleteCachedResult(t, warm)
}

func TestParseFileAnnotations_cache_warm_result_mutation_does_not_change_later_hit(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := writeAnnotationFixture(t, completeAnnotationSource)
	if _, err := ParseFileAnnotations(filename); err != nil {
		t.Fatalf("cold ParseFileAnnotations() error = %v", err)
	}
	warm, err := ParseFileAnnotations(filename)
	if err != nil {
		t.Fatalf("first warm ParseFileAnnotations() error = %v", err)
	}

	// When
	warm["Widget"].StorageMode = StorageModeGeneric
	warm["WidgetSpec"].Fields["Secret"].Storage.Hash.Algorithm = HashAlgorithmSHA256
	warm["WidgetSpec"].Fields["Name"].Index.Type = IndexTypeGIN
	later, err := ParseFileAnnotations(filename)

	// Then
	if err != nil {
		t.Fatalf("later ParseFileAnnotations() error = %v", err)
	}
	assertCompleteCachedResult(t, later)
}

func TestParseFileAnnotations_cache_invalidates_changed_content_with_same_modtime(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := writeAnnotationFixture(t, completeAnnotationSource)
	if _, err := ParseFileAnnotations(filename); err != nil {
		t.Fatalf("initial ParseFileAnnotations() error = %v", err)
	}
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	changed := `package test

// +fabrica:resource
type Widget struct{}
`
	if err := os.WriteFile(filename, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filename, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	// When
	got, err := ParseFileAnnotations(filename)

	// Then
	if err != nil {
		t.Fatalf("ParseFileAnnotations() error = %v", err)
	}
	if got["Widget"].StorageMode != StorageModeGeneric {
		t.Fatalf("Widget.StorageMode = %q, want %q", got["Widget"].StorageMode, StorageModeGeneric)
	}
	if len(got) != 1 {
		t.Fatalf("ParseFileAnnotations() returned stale types: %#v", got)
	}
}

func TestParseFileAnnotations_cache_never_returns_prior_success_for_malformed_changed_source(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := writeAnnotationFixture(t, completeAnnotationSource)
	if _, err := ParseFileAnnotations(filename); err != nil {
		t.Fatalf("initial ParseFileAnnotations() error = %v", err)
	}
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("package test\ntype Widget struct {"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filename, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	// When
	got, err := ParseFileAnnotations(filename)

	// Then
	if err == nil {
		t.Fatalf("ParseFileAnnotations() = %#v, nil; want malformed-source error", got)
	}
	if globalCache.Size() != 0 {
		t.Fatalf("global cache size after malformed source = %d, want 0", globalCache.Size())
	}
}

func TestParseFileAnnotations_cache_concurrent_hits_return_isolated_results(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := writeAnnotationFixture(t, completeAnnotationSource)
	if _, err := ParseFileAnnotations(filename); err != nil {
		t.Fatalf("initial ParseFileAnnotations() error = %v", err)
	}
	const readers = 32
	start := make(chan struct{})
	errs := make(chan error, readers)
	var workers sync.WaitGroup
	workers.Add(readers)

	// When
	for i := 0; i < readers; i++ {
		go func(cost int) {
			defer workers.Done()
			<-start
			got, err := ParseFileAnnotations(filename)
			if err != nil {
				errs <- err
				return
			}
			got["Widget"].IsResource = cost%2 == 0
			got["WidgetSpec"].Fields["Secret"].Storage.Hash.Cost = cost
		}(i)
	}
	close(start)
	workers.Wait()
	close(errs)

	// Then
	for err := range errs {
		t.Errorf("concurrent ParseFileAnnotations() error = %v", err)
	}
	got, err := ParseFileAnnotations(filename)
	if err != nil {
		t.Fatalf("final ParseFileAnnotations() error = %v", err)
	}
	assertCompleteCachedResult(t, got)
}

func assertCompleteCachedResult(t *testing.T, got map[string]*ResourceAnnotations) {
	t.Helper()
	widget := got["Widget"]
	if widget == nil {
		t.Fatalf("Widget missing from cached result: %#v", got)
	}
	if !widget.IsResource || widget.StorageMode != StorageModeDedicated {
		t.Fatalf("Widget resource metadata mutated: %#v", widget)
	}
	if len(widget.RawAnnotations) == 0 || widget.RawAnnotations[0] != "+fabrica:resource" {
		t.Fatalf("Widget raw annotations mutated: %#v", widget.RawAnnotations)
	}
	widgetSpec := got["WidgetSpec"]
	if widgetSpec == nil {
		t.Fatalf("WidgetSpec missing from cached result: %#v", got)
	}
	secret := widgetSpec.Fields["Secret"]
	if secret == nil || secret.Storage == nil || secret.Storage.Hash == nil {
		t.Fatalf("WidgetSpec.Secret missing storage: %#v", secret)
	}
	if secret.Storage.Hash.Algorithm != HashAlgorithmBcrypt || secret.Storage.Hash.Cost != 12 {
		t.Fatalf("WidgetSpec.Secret storage mutated: %#v", secret.Storage)
	}
	name := widgetSpec.Fields["Name"]
	if name == nil || name.Index == nil || len(name.RawAnnotations) == 0 {
		t.Fatalf("WidgetSpec.Name missing annotations: %#v", name)
	}
	if name.Index.Type != IndexTypeBTree || !name.Unique || name.RawAnnotations[0] != "+fabrica:field:default=guest" {
		t.Fatalf("WidgetSpec.Name annotations mutated: %#v", name)
	}
	if _, ok := got["Marker"]; !ok {
		t.Fatal("Marker type missing from cached result")
	}
}

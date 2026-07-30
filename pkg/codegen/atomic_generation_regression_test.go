// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestAtomic_preflight_validation_failure_preserves_entire_output_tree(t *testing.T) {
	// Given
	gen, root, sourcePath := newStaleSchemaGenerator(t)
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("initial GenerateEntSchemas() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte(unsupportedStaleDedicatedSource), 0o644); err != nil {
		t.Fatalf("replace source with unsupported field: %v", err)
	}
	before := snapshotAtomicTree(t, root)

	// When
	err := gen.PrepareResourceAnnotations()

	// Then
	if !errors.Is(err, annotations.ErrUnsupportedCapability) {
		t.Fatalf("PrepareResourceAnnotations() error = %v, want unsupported capability", err)
	}
	var capabilityErr *annotations.CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Filename != sourcePath || capabilityErr.Line <= 0 || capabilityErr.TypeName != "StaleDedicatedSpec" || capabilityErr.FieldName != "Value" || capabilityErr.Directive != "field declaration" {
		t.Fatalf("CapabilityError = %#v", capabilityErr)
	}
	after := snapshotAtomicTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("validation failure mutated output tree\nbefore: %#v\nafter: %#v", before, after)
	}
}

func snapshotAtomicTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[relative] = bytes.Clone(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot output tree: %v", err)
	}
	return result
}

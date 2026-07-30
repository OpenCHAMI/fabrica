// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_characterizes_existing_storage_and_index_directives(t *testing.T) {
	// Given
	filename := filepath.Join(t.TempDir(), "token.go")
	const source = `package test

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	Secret string ` + "`json:\"secret\"`" + `
	// +fabrica:field:index
	Count int ` + "`json:\"count\"`" + `
	// +fabrica:field:index=btree
	Enabled bool ` + "`json:\"enabled\"`" + `
}`
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatalf("Given source file: %v", err)
	}

	// When
	got, err := ResolveResourceAnnotations(filename, "Token")

	// Then
	if err != nil {
		t.Fatalf("ResolveResourceAnnotations() error = %v", err)
	}
	secret := got.Fields["Secret"]
	if secret == nil || secret.Storage == nil || secret.Storage.Hash == nil {
		t.Fatalf("Secret storage = %#v", secret)
	}
	if secret.Storage.Type != StorageTypeHashed || secret.Storage.Hash.Algorithm != HashAlgorithmBcrypt || secret.Storage.Hash.Cost != 12 {
		t.Fatalf("Secret storage = %#v", secret.Storage)
	}
	for _, fieldName := range []string{"Count", "Enabled"} {
		field := got.Fields[fieldName]
		if field == nil || field.Index == nil || field.Index.Type != IndexTypeBTree {
			t.Errorf("%s index = %#v", fieldName, field)
		}
	}
}

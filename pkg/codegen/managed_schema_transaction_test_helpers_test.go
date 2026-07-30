// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func canonicalSchemaRootForTest(t *testing.T, output *managedSchemaOutput) string {
	t.Helper()
	canonical, err := output.canonicalizeRoot()
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func writeTransactionArtifactFixture(t *testing.T, path string, manifest transactionManifest) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeTransactionManifest(newManagedSchemaOperations(nil), path, manifest); err != nil {
		t.Fatal(err)
	}
}

func assertNoTransactionArtifacts(t *testing.T, current string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(current))
	if err != nil {
		t.Fatal(err)
	}
	prefixes := []string{
		filepath.Base(current) + ".fabrica-staging.",
		filepath.Base(current) + ".fabrica-backup.",
		filepath.Base(current) + ".fabrica-quarantine.",
	}
	for _, entry := range entries {
		for _, prefix := range prefixes {
			if strings.HasPrefix(entry.Name(), prefix) {
				t.Errorf("transaction artifact remains: %s", entry.Name())
			}
		}
	}
}

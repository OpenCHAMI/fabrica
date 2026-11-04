// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitScaffoldsAreGofmtClean(t *testing.T) {
	type scaffoldCase struct {
		name, storage, driver string
		features              int
		noVersion             bool
	}
	cases := []scaffoldCase{
		{name: "ent-sqlite", storage: "ent", driver: "sqlite", features: 15},
		{name: "ent-postgres", storage: "ent", driver: "postgres", features: 15},
		{name: "ent-mysql", storage: "ent", driver: "mysql", features: 15},
		{name: "custom", storage: "custom", features: 15},
		{name: "no-version", storage: "file", noVersion: true},
	}
	for features := range 16 {
		// Reconciliation without events is not a supported init combination.
		if features&8 != 0 && features&4 == 0 {
			continue
		}
		cases = append(cases, scaffoldCase{name: fmt.Sprintf("file-features-%02d", features), storage: "file", features: features})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "service")
			opts := &initOptions{
				modulePath: "example.com/service", withStorage: true,
				storageType: tc.storage, dbDriver: tc.driver,
				withAuth: tc.features&1 != 0, withMetrics: tc.features&2 != 0,
				withEvents: tc.features&4 != 0, withReconcile: tc.features&8 != 0,
				withVersion: !tc.noVersion, validationMode: "strict", eventBusType: "memory",
				apiGroup: "example.com", storageVersion: "v1", apiVersions: []string{"v1"},
				reconcileWorkers: 5, reconcileRequeueMs: 5,
			}
			if err := runInit(dir, opts); err != nil {
				t.Fatal(err)
			}
			files := 0
			if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() || filepath.Ext(path) != ".go" {
					return nil
				}
				files++
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				formatted, err := format.Source(source)
				if err != nil {
					t.Errorf("format %s: %v", path, err)
				} else if !bytes.Equal(source, formatted) {
					t.Errorf("scaffold is not gofmt-clean: %s", path)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if files == 0 {
				t.Fatal("init did not create any Go source files")
			}
		})
	}
}

func TestScaffoldFormatErrorPreservesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.go")
	const original = "package main\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	// Rendering a non-Go template into a Go file must fail before truncating it.
	err := generateFromTemplate("init/readme.md.tmpl", path, templateData{ProjectName: "invalid"})
	if err == nil || !strings.Contains(err.Error(), "format") {
		t.Fatalf("format error = %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != original {
		t.Fatalf("existing file changed after format error: %q, %v", got, err)
	}
}

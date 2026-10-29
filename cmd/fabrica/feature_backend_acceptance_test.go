// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	configpkg "github.com/openchami/fabrica/internal/config"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFeatureLoaderBackendDatasetTransitions(t *testing.T) {
	h := newFeatureAcceptance(t)
	p := h.project(t, "backend-transition", "file")
	p.generate(t)
	dataset := filepath.Join(h.workspace, "preserved-dataset.json")
	originalFiles := filepath.Join(h.workspace, "original-file-data")
	database := filepath.Join(h.workspace, "original-sqlite.db")
	check := func(p *acceptanceProject, backend, mode, dataDir, db, expected string) {
		t.Helper()
		p.fixture(t, "backend_test.go.tmpl", struct{ APIImport, Backend string }{p.apiImport, backend})
		p.check(t, []string{"ACCEPTANCE_DATASET_MODE=" + mode, "ACCEPTANCE_DATA_DIR=" + dataDir, "ACCEPTANCE_DATABASE_FILE=" + db, "ACCEPTANCE_EXPECTED_DATASET=" + expected})
	}
	check(p, "file", "seed", originalFiles, database, dataset)
	sourceFiles := acceptanceTree(t, originalFiles)
	preservedDataset, err := os.ReadFile(dataset)
	if err != nil {
		t.Fatal(err)
	}
	p.cfg.Features.Storage.Type = "ent"
	p.cfg.Features.Storage.DBDriver = "sqlite3"
	if err := configpkg.SaveConfig(p.dir, p.cfg); err != nil {
		t.Fatal(err)
	}
	p.generate(t)
	check(p, "ent", "import", originalFiles, database, dataset)
	check(p, "ent", "verify", originalFiles, database, dataset)
	if !reflect.DeepEqual(sourceFiles, acceptanceTree(t, originalFiles)) {
		t.Fatal("file-to-Ent transition changed the original source files")
	}
	freshEnt := h.project(t, "fresh-ent", "ent")
	freshEnt.generate(t)
	check(freshEnt, "ent", "seed", filepath.Join(h.workspace, "unused-fresh-files"), filepath.Join(h.workspace, "fresh.db"), filepath.Join(h.workspace, "fresh-dataset.json"))
	if !reflect.DeepEqual(p.outputs(t), freshEnt.outputs(t)) {
		t.Fatal("regenerated Ent outputs differ from a fresh Ent service")
	}
	p.cfg.Features.Storage.Type = "file"
	if err := configpkg.SaveConfig(p.dir, p.cfg); err != nil {
		t.Fatal(err)
	}
	before := acceptanceTree(t, p.dir)
	output := p.run(t, p.dir, false, nil, p.binary, "generate", "--fabrica-source", h.repo)
	if !strings.Contains(string(output), "ent-to-file storage migration") {
		t.Fatalf("unsafe backend transition was not rejected: %s", output)
	}
	if !reflect.DeepEqual(before, acceptanceTree(t, p.dir)) {
		t.Fatal("rejected Ent transition changed project files")
	}
	// Preserve the previous backend outside the service. The database itself stays
	// at its original external path, so subsequent file generation cannot migrate it.
	backup := filepath.Join(h.workspace, "preserved-ent-source")
	for _, path := range []string{"internal/storage/ent", "internal/storage/ent_adapter.go", "internal/storage/ent_queries_generated.go", "internal/storage/ent_transactions_generated.go", "internal/storage/ent_revisions_generated.go", "internal/storage/generate.go", "cmd/server/export.go", "cmd/server/import.go"} {
		source := filepath.Join(p.dir, path)
		if _, err := os.Lstat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(backup, path)
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(source, destination); err != nil {
			t.Fatal(err)
		}
	}
	preservedSource := acceptanceTree(t, backup)
	sourceDatabase, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	p.generate(t)
	destinationFiles := filepath.Join(h.workspace, "new-file-data")
	check(p, "file", "import", destinationFiles, database, dataset)
	check(p, "file", "verify", destinationFiles, database, dataset)
	if !reflect.DeepEqual(sourceFiles, acceptanceTree(t, originalFiles)) {
		t.Fatal("explicit migration changed original file dataset")
	}
	if !reflect.DeepEqual(preservedSource, acceptanceTree(t, backup)) {
		t.Fatal("file regeneration changed preserved Ent source")
	}
	data, err := os.ReadFile(database)
	if err != nil || !bytes.Equal(data, sourceDatabase) {
		t.Fatalf("file backend changed the preserved SQLite database: %v", err)
	}
	data, err = os.ReadFile(dataset)
	if err != nil || !bytes.Equal(data, preservedDataset) {
		t.Fatalf("source transfer record changed: %v", err)
	}
	freshFile := h.project(t, "fresh-file-after-ent", "file")
	freshFile.generate(t)
	if !reflect.DeepEqual(p.outputs(t), freshFile.outputs(t)) {
		t.Fatal("explicitly migrated file outputs differ from a fresh file service")
	}
}

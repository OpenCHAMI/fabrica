// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreDanglingCanonicalFailsClosed(t *testing.T) {
	t.Parallel()
	store, _, _ := legacyFixture(t, "legacy")
	if err := os.Symlink(filepath.Join(store.root, "missing"), canonicalFilename(store, "legacy")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.GetSeries(context.Background(), "legacy"); err == nil || errors.Is(err, ErrSeriesNotFound) {
		t.Errorf("dangling canonical GetSeries = %v; want error, not legacy or absent", err)
	}
	if _, _, _, err := store.EnsureRevision(context.Background(), "legacy", "two", "digest-two", testResource{}); err == nil || errors.Is(err, ErrSeriesNotFound) {
		t.Errorf("dangling canonical EnsureRevision = %v; want error", err)
	}
	if _, _, err := store.CreateSeries(context.Background(), "legacy", "new", "new", testResource{}); err == nil || errors.Is(err, ErrSeriesExists) || errors.Is(err, ErrSeriesNotFound) {
		t.Errorf("dangling canonical CreateSeries = %v; want error", err)
	}
}

func TestFileStoreIncompleteCanonicalFailsClosed(t *testing.T) {
	t.Parallel()
	store, _, _ := legacyFixture(t, "legacy")
	if err := os.WriteFile(canonicalFilename(store, "legacy"), []byte(`{"status":{"name":"legacy"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.GetSeries(context.Background(), "legacy"); err == nil || errors.Is(err, ErrSeriesNotFound) {
		t.Errorf("incomplete canonical GetSeries = %v; want error", err)
	}
	if _, _, _, err := store.EnsureRevision(context.Background(), "legacy", "two", "digest-two", testResource{}); err == nil || errors.Is(err, ErrSeriesNotFound) {
		t.Errorf("incomplete canonical EnsureRevision = %v; want error without allocating", err)
	}
	if _, _, err := store.CreateSeries(context.Background(), "legacy", "new", "new", testResource{}); err == nil || errors.Is(err, ErrSeriesExists) || errors.Is(err, ErrSeriesNotFound) {
		t.Errorf("incomplete canonical CreateSeries = %v; want error", err)
	}
}

func TestFileStoreCanonicalNonRegularFailsClosed(t *testing.T) {
	for _, kind := range []string{"symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			store, _, _ := legacyFixture(t, "legacy")
			path := canonicalFilename(store, "legacy")
			var err error
			if kind == "symlink" {
				err = os.Symlink(legacyFilename(store, "legacy"), path)
			} else {
				err = os.Mkdir(path, 0o700)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.GetSeries(context.Background(), "legacy"); err == nil || errors.Is(err, ErrSeriesNotFound) {
				t.Fatalf("%s canonical GetSeries = %v; want error", kind, err)
			}
		})
	}
}

func TestFileStoreCanonicalInvariants(t *testing.T) {
	tests := []struct {
		name   string
		change func(*fileSeries)
	}{
		{"zero count", func(s *fileSeries) { s.Status.RevisionCount = 0 }},
		{"count below list length", func(s *fileSeries) { s.RevisionUIDs = append(s.RevisionUIDs, "other") }},
		{"missing list", func(s *fileSeries) { s.RevisionUIDs = nil }},
		{"empty UID", func(s *fileSeries) { s.RevisionUIDs = []string{""} }},
		{"duplicate UID", func(s *fileSeries) {
			s.RevisionUIDs = append(s.RevisionUIDs, s.RevisionUIDs[0])
			s.Status.RevisionCount = 2
		}},
		{"empty default", func(s *fileSeries) { s.Status.DefaultRevisionUID = "" }},
		{"default outside list", func(s *fileSeries) { s.Status.DefaultRevisionUID = "other" }},
		{"empty latest", func(s *fileSeries) { s.Status.LatestRevisionUID = "" }},
		{"latest outside list", func(s *fileSeries) { s.Status.LatestRevisionUID = "other" }},
		{"name binding outside list", func(s *fileSeries) { s.RevisionNames["one"] = "other" }},
		{"empty name binding", func(s *fileSeries) { s.RevisionNames["one"] = "" }},
		{"duplicate name binding UID", func(s *fileSeries) { s.RevisionNames["two"] = s.RevisionNames["one"] }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, _, _ := legacyFixture(t, "legacy")
			var series fileSeries
			if err := readJSONFile(legacyFilename(store, "legacy"), &series); err != nil {
				t.Fatal(err)
			}
			tt.change(&series)
			if err := writeJSONFile(canonicalFilename(store, "legacy"), series); err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.GetSeries(context.Background(), "legacy"); err == nil || errors.Is(err, ErrSeriesNotFound) {
				t.Fatalf("invalid canonical state accepted: %v", err)
			}
		})
	}
}

func TestFileStoreMissingRecordKeepsSeriesReadable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	record, _, err := store.CreateSeries(ctx, "legacy", "one", "digest-one", testResource{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(store.recordPath(record.Metadata.UID)); err != nil {
		t.Fatal(err)
	}
	status, _, err := store.GetSeries(ctx, "legacy")
	if err != nil || status.DefaultRevisionUID != record.Metadata.UID || status.LatestRevisionUID != record.Metadata.UID {
		t.Fatalf("series with missing record = %#v, %v", status, err)
	}
	if _, err := store.GetRevision(ctx, "legacy", record.Metadata.UID); !errors.Is(err, ErrRevisionNotFound) {
		t.Fatalf("missing record = %v, want ErrRevisionNotFound", err)
	}
}

func TestFileStoreLegacyUnnamedSeriesWithoutUIDList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := store.CreateSeries(ctx, "legacy", "", "first", testResource{})
	if err != nil {
		t.Fatal(err)
	}
	for _, digest := range []string{"second", "third"} {
		if _, _, _, err := store.EnsureRevision(ctx, "legacy", "", digest, testResource{}); err != nil {
			t.Fatal(err)
		}
	}
	var legacy fileSeries
	if err := readJSONFile(canonicalFilename(store, "legacy"), &legacy); err != nil {
		t.Fatal(err)
	}
	legacy.RevisionUIDs = nil
	legacy.RevisionNames = nil
	if err := writeJSONFile(legacyFilename(store, "legacy"), legacy); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(canonicalFilename(store, "legacy")); err != nil {
		t.Fatal(err)
	}
	status, _, err := store.GetSeries(ctx, "legacy")
	if err != nil || status.RevisionCount != 3 || status.DefaultRevisionUID != first.Metadata.UID {
		t.Fatalf("legacy unnamed status = %#v, %v", status, err)
	}
	fourth, created, _, err := store.EnsureRevision(ctx, "legacy", "", "fourth", testResource{})
	if err != nil || !created || fourth.Metadata.Number != 4 {
		t.Fatalf("legacy unnamed ensure = %#v, %t, %v", fourth, created, err)
	}
	status, _, err = store.GetSeries(ctx, "legacy")
	if err != nil || status.RevisionCount != 4 || status.DefaultRevisionUID != first.Metadata.UID || status.LatestRevisionUID != fourth.Metadata.UID {
		t.Fatalf("canonicalized unnamed status = %#v, %v", status, err)
	}
}

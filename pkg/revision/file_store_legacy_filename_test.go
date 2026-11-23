// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func legacyFilename(store *FileStore[testResource], name string) string {
	return filepath.Join(store.root, "series", base64.RawURLEncoding.EncodeToString([]byte(name))+".json")
}

func canonicalFilename(store *FileStore[testResource], name string) string {
	return filepath.Join(store.root, "series", fmt.Sprintf("sha256.%x.json", sha256.Sum256([]byte(name))))
}

func legacyFixture(t *testing.T, name string) (*FileStore[testResource], Record[testResource], string) {
	t.Helper()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := store.CreateSeriesWithStatus(context.Background(), name, "one", "digest-one", testResource{Value: "first"}, testResource{Value: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if store.seriesPath(name) != legacyFilename(store, name) {
		if err := os.Rename(store.seriesPath(name), legacyFilename(store, name)); err != nil {
			t.Fatal(err)
		}
	}
	_, etag, err := store.GetSeries(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	return store, first, etag
}

func TestFileStoreLegacyReadAndMutation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, first, etag := legacyFixture(t, "legacy")
	legacyBefore, err := os.ReadFile(legacyFilename(store, "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	recordBefore, err := os.ReadFile(store.recordPath(first.Metadata.UID))
	if err != nil {
		t.Fatal(err)
	}
	status, gotETag, err := store.GetSeries(ctx, "legacy")
	if err != nil || gotETag != etag || status.DefaultRevisionUID != first.Metadata.UID {
		t.Fatalf("legacy status = %#v, %q, %v", status, gotETag, err)
	}
	if _, _, err := store.CreateSeries(ctx, "legacy", "other", "other", testResource{}); !errors.Is(err, ErrSeriesExists) {
		t.Fatalf("legacy create = %v", err)
	}
	retry, created, _, err := store.EnsureRevision(ctx, "legacy", "one", "digest-one", testResource{})
	if err != nil || created || retry.Metadata.UID != first.Metadata.UID || retry.Metadata.Digest != first.Metadata.Digest || retry.Metadata.Number != first.Metadata.Number {
		t.Fatalf("legacy ensure = %#v, %t, %v", retry, created, err)
	}
	second, created, _, err := store.EnsureRevision(ctx, "legacy", "two", "digest-two", testResource{Value: "second"})
	if err != nil || !created || second.Metadata.Number != 2 {
		t.Fatalf("legacy mutation = %#v, %t, %v", second, created, err)
	}
	legacyAfter, err := os.ReadFile(legacyFilename(store, "legacy"))
	if err != nil || !bytes.Equal(legacyBefore, legacyAfter) {
		t.Fatalf("legacy file changed: %v", err)
	}
	recordAfter, err := os.ReadFile(store.recordPath(first.Metadata.UID))
	if err != nil || !bytes.Equal(recordBefore, recordAfter) {
		t.Fatalf("original record changed: %v", err)
	}
	if _, err := os.Stat(canonicalFilename(store, "legacy")); err != nil {
		t.Fatalf("canonical state: %v", err)
	}
	reopened, err := NewFileStore[testResource](filepath.Dir(filepath.Dir(store.root)), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Resolve(ctx, Reference{Name: "legacy", RevisionName: "one"})
	if err != nil || got.Metadata.UID != first.Metadata.UID || got.Metadata.Digest != first.Metadata.Digest || got.Metadata.Number != first.Metadata.Number {
		t.Fatalf("reopened legacy revision = %#v, %v", got, err)
	}
	status, _, err = reopened.GetSeries(ctx, "legacy")
	if err != nil || status.DefaultRevisionUID != first.Metadata.UID || status.LatestRevisionUID != second.Metadata.UID || status.RevisionCount != 2 {
		t.Fatalf("reopened state = %#v, %v", status, err)
	}
	var resourceStatus testResource
	if err := reopened.GetStatus(ctx, "legacy", &resourceStatus); err != nil || resourceStatus.Value != "status" {
		t.Fatalf("legacy resource status = %#v, %v", resourceStatus, err)
	}
	_, currentETag, err := reopened.GetSeries(ctx, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.PromoteDefault(ctx, "legacy", second.Metadata.UID, currentETag); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.RetireRevision(ctx, "legacy", first.Metadata.UID); err != nil {
		t.Fatal(err)
	}
	list, err := reopened.ListRevisions(ctx, "legacy")
	if err != nil || len(list) != 2 || list[0].Metadata.UID != first.Metadata.UID || list[0].Metadata.RetiredAt == nil {
		t.Fatalf("legacy list = %#v, %v", list, err)
	}
}

func TestFileStoreLegacyAliasDoesNotLeak(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, first, _ := legacyFixture(t, "aaG")
	// Simulate a case-insensitive alias even on case-sensitive filesystems.
	if err := os.Rename(legacyFilename(store, "aaG"), legacyFilename(store, "aaa")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.GetSeries(ctx, "aaa"); !errors.Is(err, ErrSeriesNotFound) {
		t.Fatalf("aliased legacy read = %v", err)
	}
	second, _, err := store.CreateSeries(ctx, "aaa", "one", "second", testResource{})
	if err != nil || second.Metadata.UID == first.Metadata.UID {
		t.Fatalf("aliased create = %#v, %v", second, err)
	}
	if err := os.Rename(legacyFilename(store, "aaa"), legacyFilename(store, "aaG")); err != nil {
		t.Fatal(err)
	}
	got, err := store.Resolve(ctx, Reference{Name: "aaG"})
	if err != nil || got.Metadata.UID != first.Metadata.UID {
		t.Fatalf("original legacy = %#v, %v", got, err)
	}
}

func TestFileStoreCanonicalCorruptionDoesNotFallback(t *testing.T) {
	t.Parallel()
	store, first, etag := legacyFixture(t, "legacy")
	ctx := context.Background()
	for _, data := range []string{"{", `{"status":{"name":"other"}}`} {
		if err := os.WriteFile(canonicalFilename(store, "legacy"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		checks := map[string]func() error{
			"series": func() error { _, _, err := store.GetSeries(ctx, "legacy"); return err },
			"create": func() error { _, _, err := store.CreateSeries(ctx, "legacy", "new", "new", testResource{}); return err },
			"ensure": func() error {
				_, _, _, err := store.EnsureRevision(ctx, "legacy", "new", "new", testResource{})
				return err
			},
			"resolve":       func() error { _, err := store.Resolve(ctx, Reference{Name: "legacy"}); return err },
			"get":           func() error { _, err := store.GetRevision(ctx, "legacy", first.Metadata.UID); return err },
			"list":          func() error { _, err := store.ListRevisions(ctx, "legacy"); return err },
			"promote":       func() error { _, err := store.PromoteDefault(ctx, "legacy", first.Metadata.UID, etag); return err },
			"retire":        func() error { _, err := store.RetireRevision(ctx, "legacy", first.Metadata.UID); return err },
			"get status":    func() error { return store.GetStatus(ctx, "legacy", &testResource{}) },
			"update status": func() error { _, err := store.UpdateStatus(ctx, "legacy", testResource{}); return err },
		}
		for operation, run := range checks {
			err := run()
			if err == nil || errors.Is(err, ErrSeriesNotFound) || errors.Is(err, ErrSeriesExists) {
				t.Fatalf("%s with canonical %q returned %v", operation, data, err)
			}
		}
	}
}

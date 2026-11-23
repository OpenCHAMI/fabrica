// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileStoreLongNamePersists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	name := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	store, err := NewFileStore[testResource](dir, "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := store.CreateSeriesWithStatus(ctx, name, "one", "digest-one", testResource{Value: "first"}, testResource{Value: "status"})
	if err != nil {
		t.Fatalf("create 253-byte series: %v", err)
	}
	store, err = NewFileStore[testResource](dir, "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	var status testResource
	if err := store.GetStatus(ctx, name, &status); err != nil || status.Value != "status" {
		t.Fatalf("status = %#v, %v", status, err)
	}
	if _, err := store.UpdateStatus(ctx, name, testResource{Value: "updated"}); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetRevision(ctx, name, first.Metadata.UID); err != nil || got.Metadata.Digest != "digest-one" {
		t.Fatalf("get revision = %#v, %v", got, err)
	}
	second, created, etag, err := store.EnsureRevision(ctx, name, "two", "digest-two", testResource{Value: "second"})
	if err != nil || !created || second.Metadata.Number != 2 {
		t.Fatalf("ensure = %#v, %t, %v", second, created, err)
	}
	for _, ref := range []Reference{{Name: name}, {Name: name, Selector: SelectorDefault}, {Name: name, UID: first.Metadata.UID}, {Name: name, RevisionName: "one"}, {Name: name, Selector: SelectorLatest}} {
		got, err := store.Resolve(ctx, ref)
		want := first.Metadata.UID
		if ref.Selector == SelectorLatest {
			want = second.Metadata.UID
		}
		if err != nil || got.Metadata.UID != want {
			t.Fatalf("resolve %#v = %#v, %v", ref, got, err)
		}
	}
	if _, err := store.PromoteDefault(ctx, name, second.Metadata.UID, etag); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RetireRevision(ctx, name, first.Metadata.UID); err != nil {
		t.Fatal(err)
	}
	records, err := store.ListRevisions(ctx, name)
	if err != nil || len(records) != 2 || records[0].Metadata.RetiredAt == nil {
		t.Fatalf("list = %#v, %v", records, err)
	}
}

func TestFileStoreCaseDistinctNames(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := store.CreateSeries(ctx, "aaG", "one", "first", testResource{})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := store.CreateSeries(ctx, "aaa", "one", "second", testResource{})
	if err != nil {
		t.Fatalf("create case-distinct series: %v", err)
	}
	if first.Metadata.UID == second.Metadata.UID {
		t.Fatal("distinct series share UID")
	}
	for _, name := range []string{"aaG", "aaa"} {
		got, created, _, err := store.EnsureRevision(ctx, name, "two", name, testResource{})
		if err != nil || !created || got.Metadata.Number != 2 || got.Metadata.SeriesName != name {
			t.Fatalf("ensure %q = %#v, %t, %v", name, got, created, err)
		}
		status, _, err := store.GetSeries(ctx, name)
		if err != nil || status.RevisionCount != 2 || status.LatestRevisionUID != got.Metadata.UID {
			t.Fatalf("status %q = %#v, %v", name, status, err)
		}
	}
}

func TestFileStoreFilenamePortable(t *testing.T) {
	t.Parallel()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aaG", "aaa", "a", strings.Repeat("x", 253), "UPPER.case-name"} {
		filename := filepath.Base(store.seriesPath(name))
		if len(filename) != 76 || filename != strings.ToLower(filename) || !strings.HasPrefix(filename, "sha256.") || !strings.HasSuffix(filename, ".json") {
			t.Errorf("filename %q for %q is not bounded lowercase canonical form", filename, name)
		}
	}
	if strings.EqualFold(filepath.Base(store.seriesPath("aaG")), filepath.Base(store.seriesPath("aaa"))) {
		t.Fatal("case-distinct names collide under case folding")
	}
	for length := 1; length <= 253; length++ {
		name := strings.Repeat("A", length)
		filename := filepath.Base(store.seriesPath(name))
		want := fmt.Sprintf("sha256.%x.json", sha256.Sum256([]byte(name)))
		if filename != want || len(filename) != 76 || filename != strings.ToLower(filename) {
			t.Fatalf("filename for length %d = %q, want %q", length, filename, want)
		}
	}
}

func TestFileStoreFilenameScope(t *testing.T) {
	t.Parallel()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	record, _, err := store.CreateSeries(context.Background(), "aaG", "one", "digest", testResource{})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(store.root, "records"))
	if err != nil || len(entries) != 1 || entries[0].Name() != record.Metadata.UID+".json" {
		t.Fatalf("record files = %v, %v", entries, err)
	}
	entries, err = os.ReadDir(filepath.Join(store.root, "series"))
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(canonicalFilename(store, "aaG")) {
		t.Fatalf("series files = %v, %v", entries, err)
	}
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type testResource struct {
	Value string `json:"value"`
}

func TestFileStoreRevisionWorkflow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	first, etag1, err := store.CreateSeries(ctx, "production", "release-a", "sha256:first", testResource{Value: "first"})
	if err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}
	if first.Metadata.Number != 1 {
		t.Fatalf("first revision number = %d, want 1", first.Metadata.Number)
	}
	if !strings.HasPrefix(first.Metadata.UID, "rev_") {
		t.Fatalf("first revision UID = %q, want rev_ prefix", first.Metadata.UID)
	}

	series, gotETag, err := store.GetSeries(ctx, "production")
	if err != nil {
		t.Fatalf("GetSeries() error = %v", err)
	}
	if gotETag != etag1 {
		t.Fatalf("series ETag = %q, want %q", gotETag, etag1)
	}
	if series.DefaultRevisionUID != first.Metadata.UID || series.LatestRevisionUID != first.Metadata.UID {
		t.Fatalf("initial aliases = default %q latest %q, want %q", series.DefaultRevisionUID, series.LatestRevisionUID, first.Metadata.UID)
	}

	if _, _, err := store.CreateSeries(ctx, "production", "other", "sha256:other", testResource{}); !errors.Is(err, ErrSeriesExists) {
		t.Fatalf("duplicate CreateSeries() error = %v, want ErrSeriesExists", err)
	}

	retried, created, _, err := store.EnsureRevision(ctx, "production", "release-a", "sha256:first", testResource{Value: "first"})
	if err != nil {
		t.Fatalf("idempotent EnsureRevision() error = %v", err)
	}
	if created || retried.Metadata.UID != first.Metadata.UID {
		t.Fatalf("idempotent ensure = created %t UID %q, want false and %q", created, retried.Metadata.UID, first.Metadata.UID)
	}

	if _, _, _, err := store.EnsureRevision(ctx, "production", "release-a", "sha256:changed", testResource{Value: "changed"}); !errors.Is(err, ErrRevisionNameConflict) {
		t.Fatalf("conflicting EnsureRevision() error = %v, want ErrRevisionNameConflict", err)
	}

	second, created, etag2, err := store.EnsureRevision(ctx, "production", "release-b", "sha256:second", testResource{Value: "second"})
	if err != nil {
		t.Fatalf("second EnsureRevision() error = %v", err)
	}
	if !created || second.Metadata.Number != 2 {
		t.Fatalf("second ensure = created %t number %d, want true and 2", created, second.Metadata.Number)
	}

	series, _, err = store.GetSeries(ctx, "production")
	if err != nil {
		t.Fatalf("GetSeries() after ensure error = %v", err)
	}
	if series.LatestRevisionUID != second.Metadata.UID || series.DefaultRevisionUID != first.Metadata.UID {
		t.Fatalf("aliases after ensure = default %q latest %q", series.DefaultRevisionUID, series.LatestRevisionUID)
	}

	bare, err := store.Resolve(ctx, Reference{Name: "production"})
	if err != nil {
		t.Fatalf("Resolve(bare) error = %v", err)
	}
	if bare.Metadata.UID != first.Metadata.UID {
		t.Fatalf("Resolve(bare) UID = %q, want default %q", bare.Metadata.UID, first.Metadata.UID)
	}

	latest, err := store.Resolve(ctx, Reference{Name: "production", Selector: SelectorLatest})
	if err != nil {
		t.Fatalf("Resolve(latest) error = %v", err)
	}
	if latest.Metadata.UID != second.Metadata.UID {
		t.Fatalf("Resolve(latest) UID = %q, want %q", latest.Metadata.UID, second.Metadata.UID)
	}

	byName, err := store.Resolve(ctx, Reference{Name: "production", RevisionName: "release-b"})
	if err != nil {
		t.Fatalf("Resolve(revisionName) error = %v", err)
	}
	if byName.Metadata.UID != second.Metadata.UID {
		t.Fatalf("Resolve(revisionName) UID = %q, want %q", byName.Metadata.UID, second.Metadata.UID)
	}

	promoted, etag3, err := store.PromoteDefault(ctx, "production", second.Metadata.UID, etag2)
	if err != nil {
		t.Fatalf("PromoteDefault() error = %v", err)
	}
	if promoted.DefaultRevisionUID != second.Metadata.UID || etag3 == etag2 {
		t.Fatalf("promotion = default %q etag %q, want %q and changed ETag", promoted.DefaultRevisionUID, etag3, second.Metadata.UID)
	}

	if _, _, err := store.PromoteDefault(ctx, "production", first.Metadata.UID, etag2); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("stale PromoteDefault() error = %v, want ErrPreconditionFailed", err)
	}
	series, _, err = store.GetSeries(ctx, "production")
	if err != nil {
		t.Fatalf("GetSeries() after stale promotion error = %v", err)
	}
	if series.DefaultRevisionUID != second.Metadata.UID {
		t.Fatalf("default after stale promotion = %q, want %q", series.DefaultRevisionUID, second.Metadata.UID)
	}
}

func TestFileStoreDoesNotRebindRevisionNumbersAfterReopen(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	store, err := NewFileStore[testResource](dir, "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	if _, _, err := store.CreateSeries(ctx, "production", "one", "sha256:one", testResource{}); err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}

	reopened, err := NewFileStore[testResource](dir, "BootConfig")
	if err != nil {
		t.Fatalf("reopen NewFileStore() error = %v", err)
	}
	record, created, _, err := reopened.EnsureRevision(ctx, "production", "two", "sha256:two", testResource{})
	if err != nil {
		t.Fatalf("EnsureRevision() after reopen error = %v", err)
	}
	if !created || record.Metadata.Number != 2 {
		t.Fatalf("revision after reopen = created %t number %d, want true and 2", created, record.Metadata.Number)
	}
}

func TestFileStoreIgnoresUncommittedOrphanRecord(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	if _, _, err := store.CreateSeries(ctx, "production", "one", "sha256:one", testResource{}); err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}
	orphan := Record[testResource]{
		Metadata: Metadata{UID: "rev_01h2xcejqtf2nbrexx3vqjhp42", SeriesName: "production", Number: 2, Digest: "sha256:orphan", CreatedAt: time.Now().UTC()},
		Resource: testResource{Value: "orphan"},
	}
	if err := store.writeRecord(orphan); err != nil {
		t.Fatalf("write orphan record: %v", err)
	}

	records, err := store.ListRevisions(ctx, "production")
	if err != nil {
		t.Fatalf("ListRevisions() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("ListRevisions() count = %d, want 1 committed revision", len(records))
	}
	if _, err := store.GetRevision(ctx, "production", orphan.Metadata.UID); !errors.Is(err, ErrRevisionNotFound) {
		t.Fatalf("GetRevision(orphan) error = %v, want ErrRevisionNotFound", err)
	}
}

func TestFileStoreInstancesSerializeRevisionAllocation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	firstStore, err := NewFileStore[testResource](dir, "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore(first) error = %v", err)
	}
	secondStore, err := NewFileStore[testResource](dir, "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore(second) error = %v", err)
	}
	if _, _, err := firstStore.CreateSeries(ctx, "production", "one", "sha256:one", testResource{}); err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}

	type result struct {
		record Record[testResource]
		err    error
	}
	results := make(chan result, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	start := make(chan struct{})
	ensure := func(store *FileStore[testResource], name string) {
		defer ready.Done()
		<-start
		record, _, _, err := store.EnsureRevision(ctx, "production", name, "sha256:"+name, testResource{Value: name})
		results <- result{record: record, err: err}
	}
	go ensure(firstStore, "two")
	go ensure(secondStore, "three")
	close(start)
	ready.Wait()
	close(results)

	numbers := map[uint64]bool{}
	for got := range results {
		if got.err != nil {
			t.Fatalf("EnsureRevision() error = %v", got.err)
		}
		numbers[got.record.Metadata.Number] = true
	}
	if !numbers[2] || !numbers[3] || len(numbers) != 2 {
		t.Fatalf("allocated revision numbers = %v, want 2 and 3", numbers)
	}
}

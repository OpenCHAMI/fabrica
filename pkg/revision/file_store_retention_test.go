// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestFileStoreRetireRevisionPreservesIdentityAndSequence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	first, _, err := store.CreateSeries(ctx, "production", "one", "sha256:one", testResource{Value: "one"})
	if err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}
	second, _, etag, err := store.EnsureRevision(ctx, "production", "two", "sha256:two", testResource{Value: "two"})
	if err != nil {
		t.Fatalf("EnsureRevision(two) error = %v", err)
	}
	if _, _, err := store.PromoteDefault(ctx, "production", second.Metadata.UID, etag); err != nil {
		t.Fatalf("PromoteDefault() error = %v", err)
	}

	retired, err := store.RetireRevision(ctx, "production", first.Metadata.UID)
	if err != nil {
		t.Fatalf("RetireRevision() error = %v", err)
	}
	if retired.Metadata.RetiredAt == nil {
		t.Fatal("RetireRevision() did not record a tombstone timestamp")
	}
	if _, err := store.GetRevision(ctx, "production", first.Metadata.UID); !errors.Is(err, ErrRevisionRetired) {
		t.Fatalf("GetRevision(retired) error = %v, want ErrRevisionRetired", err)
	}
	if _, err := store.Resolve(ctx, Reference{Name: "production", RevisionName: "one"}); !errors.Is(err, ErrRevisionRetired) {
		t.Fatalf("Resolve(retired name) error = %v, want ErrRevisionRetired", err)
	}
	if _, _, _, err := store.EnsureRevision(ctx, "production", "one", "sha256:replacement", testResource{Value: "replacement"}); !errors.Is(err, ErrRevisionNameConflict) {
		t.Fatalf("EnsureRevision(rebind retired name) error = %v, want ErrRevisionNameConflict", err)
	}

	third, created, _, err := store.EnsureRevision(ctx, "production", "three", "sha256:three", testResource{Value: "three"})
	if err != nil {
		t.Fatalf("EnsureRevision(three) error = %v", err)
	}
	if !created || third.Metadata.Number != 3 {
		t.Fatalf("revision after retirement = created %t number %d, want true and 3", created, third.Metadata.Number)
	}
	records, err := store.ListRevisions(ctx, "production")
	if err != nil {
		t.Fatalf("ListRevisions() error = %v", err)
	}
	if len(records) != 3 || records[0].Metadata.UID != first.Metadata.UID || records[0].Metadata.RetiredAt == nil {
		t.Fatalf("ListRevisions() did not preserve tombstone: %#v", records)
	}
}

func TestFileStoreRejectsRetiringReferencedRevision(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	first, _, err := store.CreateSeries(ctx, "production", "one", "sha256:one", testResource{})
	if err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}
	if _, err := store.RetireRevision(ctx, "production", first.Metadata.UID); !errors.Is(err, ErrRevisionInUse) {
		t.Fatalf("RetireRevision(default/latest) error = %v, want ErrRevisionInUse", err)
	}
}

func TestFileStoreConcurrentNamedEnsureAndPromotion(t *testing.T) {
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
	first, _, err := firstStore.CreateSeries(ctx, "production", "one", "sha256:one", testResource{Value: "one"})
	if err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}

	type ensureResult struct {
		record  Record[testResource]
		created bool
		etag    string
		err     error
	}
	ensureResults := make(chan ensureResult, 2)
	var ensureWG sync.WaitGroup
	start := make(chan struct{})
	for _, store := range []*FileStore[testResource]{firstStore, secondStore} {
		ensureWG.Add(1)
		go func() {
			defer ensureWG.Done()
			<-start
			record, created, etag, err := store.EnsureRevision(ctx, "production", "two", "sha256:two", testResource{Value: "two"})
			ensureResults <- ensureResult{record: record, created: created, etag: etag, err: err}
		}()
	}
	close(start)
	ensureWG.Wait()
	close(ensureResults)

	createdCount := 0
	var second Record[testResource]
	var etag string
	for got := range ensureResults {
		if got.err != nil {
			t.Fatalf("EnsureRevision() error = %v", got.err)
		}
		if got.created {
			createdCount++
		}
		if second.Metadata.UID == "" {
			second = got.record
		} else if got.record.Metadata.UID != second.Metadata.UID {
			t.Fatalf("same revision name produced UIDs %q and %q", second.Metadata.UID, got.record.Metadata.UID)
		}
		etag = got.etag
	}
	if createdCount != 1 {
		t.Fatalf("concurrent named ensure created count = %d, want 1", createdCount)
	}

	promotionResults := make(chan error, 2)
	var promotionWG sync.WaitGroup
	for _, uid := range []string{first.Metadata.UID, second.Metadata.UID} {
		promotionWG.Add(1)
		go func() {
			defer promotionWG.Done()
			_, _, err := firstStore.PromoteDefault(ctx, "production", uid, etag)
			promotionResults <- err
		}()
	}
	promotionWG.Wait()
	close(promotionResults)
	succeeded, preconditioned := 0, 0
	for err := range promotionResults {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrPreconditionFailed):
			preconditioned++
		default:
			t.Fatalf("PromoteDefault() error = %v", err)
		}
	}
	if succeeded != 1 || preconditioned != 1 {
		t.Fatalf("concurrent promotions succeeded = %d, preconditioned = %d", succeeded, preconditioned)
	}
}

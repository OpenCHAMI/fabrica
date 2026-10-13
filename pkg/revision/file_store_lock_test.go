// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func TestFileStoreInstancesShareLockForEquivalentRelativeAndAbsoluteRoots(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	absoluteDir := t.TempDir()
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	relativeDir, err := filepath.Rel(workingDir, absoluteDir)
	if err != nil {
		t.Fatalf("Rel() error = %v", err)
	}
	absoluteStore, err := NewFileStore[testResource](absoluteDir, "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore(absolute) error = %v", err)
	}
	relativeStore, err := NewFileStore[testResource](relativeDir, "BootConfig")
	if err != nil {
		t.Fatalf("NewFileStore(relative) error = %v", err)
	}
	if absoluteStore.mu != relativeStore.mu {
		t.Fatal("equivalent store roots use different mutexes")
	}
	if _, _, err := absoluteStore.CreateSeries(ctx, "production", "one", "sha256:one", testResource{Value: "one"}); err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}

	type result struct {
		revisionName string
		record       Record[testResource]
		err          error
	}
	const concurrentRevisions = 16
	results := make(chan result, concurrentRevisions)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for worker := range concurrentRevisions {
		workers.Go(func() {
			<-start
			revisionName := "revision-" + strconv.Itoa(worker)
			record, _, _, err := []*FileStore[testResource]{absoluteStore, relativeStore}[worker%2].EnsureRevision(ctx, "production", revisionName, "sha256:"+revisionName, testResource{Value: revisionName})
			results <- result{revisionName: revisionName, record: record, err: err}
		})
	}
	close(start)
	workers.Wait()
	close(results)

	successful := make(map[string]Record[testResource], concurrentRevisions)
	for got := range results {
		if got.err != nil {
			t.Fatalf("EnsureRevision(%q) error = %v", got.revisionName, got.err)
		}
		successful[got.revisionName] = got.record
	}
	records, err := absoluteStore.ListRevisions(ctx, "production")
	if err != nil {
		t.Fatalf("ListRevisions() error = %v", err)
	}
	if len(records) != len(successful)+1 {
		t.Fatalf("ListRevisions() count = %d, want %d", len(records), len(successful)+1)
	}
	for revisionName, expected := range successful {
		got, err := relativeStore.Resolve(ctx, Reference{Name: "production", RevisionName: revisionName})
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", revisionName, err)
		}
		if got.Metadata.UID != expected.Metadata.UID {
			t.Fatalf("Resolve(%q) UID = %q, want %q", revisionName, got.Metadata.UID, expected.Metadata.UID)
		}
	}
}

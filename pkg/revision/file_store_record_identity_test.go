// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestFileStoreRejectsCopiedRecordWithDifferentUID(t *testing.T) {
	for _, operation := range []string{"get", "empty", "retired", "list", "uid", "name", "latest", "default", "ensure", "promote"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
			if err != nil {
				t.Fatal(err)
			}
			first, _, err := store.CreateSeries(ctx, "production", "a", "digest-a", testResource{Value: "first"})
			if err != nil {
				t.Fatal(err)
			}
			second, _, etag, err := store.EnsureRevision(ctx, "production", "b", "digest-b", testResource{Value: "second"})
			if err != nil {
				t.Fatal(err)
			}
			if operation == "default" {
				if _, err := store.PromoteDefault(ctx, "production", second.Metadata.UID, etag); err != nil {
					t.Fatal(err)
				}
			}
			before, etag, err := store.GetSeries(ctx, "production")
			if err != nil {
				t.Fatal(err)
			}
			first.Metadata.RevisionName = "b"
			if operation == "empty" {
				first.Metadata.UID = ""
			}
			if operation == "retired" {
				retiredAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
				first.Metadata.RetiredAt = &retiredAt
			}
			if err := writeJSONFile(store.recordPath(second.Metadata.UID), first); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "get", "empty", "retired":
				_, err = store.GetRevision(ctx, "production", second.Metadata.UID)
			case "list":
				_, err = store.ListRevisions(ctx, "production")
			case "uid":
				_, err = store.Resolve(ctx, Reference{Name: "production", UID: second.Metadata.UID})
			case "name":
				_, err = store.Resolve(ctx, Reference{Name: "production", RevisionName: "b"})
			case "latest":
				_, err = store.Resolve(ctx, Reference{Name: "production", Selector: SelectorLatest})
			case "default":
				_, err = store.Resolve(ctx, Reference{Name: "production"})
			case "ensure":
				_, _, _, err = store.EnsureRevision(ctx, "production", "b", "digest-a", testResource{})
			case "promote":
				_, err = store.PromoteDefault(ctx, "production", second.Metadata.UID, etag)
			}
			if !errors.Is(err, ErrRevisionIdentityMismatch) || errors.Is(err, ErrRevisionNotFound) || errors.Is(err, ErrRevisionRetired) {
				t.Errorf("%s error = %v, want ErrRevisionIdentityMismatch only", operation, err)
			}
			after, afterETag, err := store.GetSeries(ctx, "production")
			if err != nil || after.DefaultRevisionUID != before.DefaultRevisionUID || after.LatestRevisionUID != before.LatestRevisionUID || after.RevisionCount != before.RevisionCount || afterETag != etag {
				t.Errorf("%s changed series state: before %#v, after %#v, err %v", operation, before, after, err)
			}
		})
	}
}

func TestFileStoreOtherRecordErrorsRemainDistinct(t *testing.T) {
	for _, scenario := range []string{"unknown", "missing", "retired", "cancelled", "io"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store, err := NewFileStore[testResource](t.TempDir(), "BootConfig")
			if err != nil {
				t.Fatal(err)
			}
			first, _, err := store.CreateSeries(ctx, "production", "a", "digest-a", testResource{})
			if err != nil {
				t.Fatal(err)
			}
			uid := first.Metadata.UID
			var want error
			switch scenario {
			case "unknown":
				uid, err = NewUID()
				if err != nil {
					t.Fatal(err)
				}
				want = ErrRevisionNotFound
			case "missing":
				if err := os.Remove(store.recordPath(uid)); err != nil {
					t.Fatal(err)
				}
				want = ErrRevisionNotFound
			case "retired":
				second, _, etag, err := store.EnsureRevision(ctx, "production", "b", "digest-b", testResource{})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.PromoteDefault(ctx, "production", second.Metadata.UID, etag); err != nil {
					t.Fatal(err)
				}
				if _, err := store.RetireRevision(ctx, "production", uid); err != nil {
					t.Fatal(err)
				}
				want = ErrRevisionRetired
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
				want = context.Canceled
			case "io":
				if err := os.Remove(store.recordPath(uid)); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(store.recordPath(uid), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			_, err = store.GetRevision(ctx, "production", uid)
			if scenario == "io" {
				var pathErr *os.PathError
				if !errors.As(err, &pathErr) || errors.Is(err, ErrRevisionNotFound) {
					t.Fatalf("I/O error = %v, want filesystem error", err)
				}
			} else if !errors.Is(err, want) {
				t.Fatalf("%s error = %v, want %v", scenario, err, want)
			}
			if errors.Is(err, ErrRevisionIdentityMismatch) {
				t.Fatalf("%s error misclassified as identity mismatch: %v", scenario, err)
			}
		})
	}
}

func TestFileStoreMatchingLegacyRecordUID(t *testing.T) {
	for _, kind := range []string{"typeid", "uuid"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			store, record, _ := legacyFixture(t, "legacy")
			if kind == "uuid" {
				var series fileSeries
				if err := readJSONFile(legacyFilename(store, "legacy"), &series); err != nil {
					t.Fatal(err)
				}
				record.Metadata.UID = "01890a29-5f5a-7000-8000-000000000001"
				series.Status.DefaultRevisionUID = record.Metadata.UID
				series.Status.LatestRevisionUID = record.Metadata.UID
				series.RevisionUIDs = []string{record.Metadata.UID}
				series.RevisionNames["one"] = record.Metadata.UID
				if err := writeJSONFile(legacyFilename(store, "legacy"), series); err != nil {
					t.Fatal(err)
				}
				if err := writeJSONFile(store.recordPath(record.Metadata.UID), record); err != nil {
					t.Fatal(err)
				}
			}
			got, err := store.GetRevision(context.Background(), "legacy", record.Metadata.UID)
			if err != nil || got.Metadata.UID != record.Metadata.UID || got.Metadata.Digest != record.Metadata.Digest {
				t.Fatalf("legacy %s record = %#v, %v", kind, got, err)
			}
		})
	}
}

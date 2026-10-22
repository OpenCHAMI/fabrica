<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# [Bug]: A spec update can recreate snapshots after resource deletion completes

## Summary

Generated file-storage services with per-resource spec snapshots can retain a snapshot for a deleted resource when spec PUT/PATCH overlaps resource DELETE. Snapshot creation occurs after the resource mutation commits. A pending writer can resume after DELETE finishes snapshot cleanup and recreate the deleted resource's snapshot directory.

This is a pre-existing snapshot lifecycle race, identified while reviewing [PR #133](https://github.com/OpenCHAMI/fabrica/pull/133). That PR prevents stale snapshot bookkeeping from resurrecting the resource, but its snapshot guard does not cover the interval between committing a spec mutation and creating the snapshot. Track lifecycle coordination separately from [issue #128](https://github.com/OpenCHAMI/fabrica/issues/128), which concerns atomic If-Match enforcement.

## Upstream tracking checked

On 2026-10-22, inspected all 33 upstream issues, open and closed, and searched PR titles and bodies for snapshot-related work. The related results were [PR #17](https://github.com/OpenCHAMI/fabrica/pull/17), which introduced spec snapshots, [issue #114](https://github.com/OpenCHAMI/fabrica/issues/114), which concerns immutable revisions, [issue #130](https://github.com/OpenCHAMI/fabrica/issues/130), which concerns mutable methods generated alongside revision methods, and PR #133. Also checked the review/general comments on PR #133 and comments on issue #128; those discussions contained no comments. No existing report tracking this specific lifecycle race was found.

## Environment

- Reproduced at commit `c93737b695dccdde392269836ea8c249ff6c2398`, containing PR #133 and its validation-lock follow-up.
- Go 1.26.10.
- Generated file storage with per-resource snapshots enabled (`Tags: map[string]string{"versioning": "enabled"}` in the existing generator fixture).
- Deterministic generated-handler reproduction; the handler test runs under the race detector.

## Reproduction

Use the generated Node handler fixture in `pkg/codegen/testdata/conditional_handlers_runtime/handlers_generated_test.go.fixture`. Its imports already provide the packages used below. Add this test and backend wrapper to that source fixture, then run:

```bash
GOTOOLCHAIN=go1.26.10 go test ./pkg/codegen \
  -run '^TestGeneratedReadHandlersExecuteConditionalRequests$' -count=1
```

The wrapper pauses the first successful mutation after its commit and before the handler can create its snapshot. A concurrent DELETE can then complete deterministically.

```go
type pausedSnapshotBackend struct {
    fabricaStorage.StorageBackend
    paused atomic.Bool
    committed, resume chan struct{}
}

func (b *pausedSnapshotBackend) Mutate(
    ctx context.Context, kind, uid string, fn fabricaStorage.Mutation,
) (json.RawMessage, error) {
    raw, err := b.StorageBackend.(fabricaStorage.AtomicMutationBackend).
        Mutate(ctx, kind, uid, fn)
    if err == nil && b.paused.CompareAndSwap(false, true) {
        close(b.committed)
        <-b.resume
    }
    return raw, err
}

func TestResourceDeletionCannotLeaveLateSnapshot(t *testing.T) {
    storage.SeedNode(testNode("initial"))
    backend := &pausedSnapshotBackend{
        StorageBackend: storage.Backend,
        committed: make(chan struct{}),
        resume: make(chan struct{}),
    }
    storage.Backend = backend

    request := httptest.NewRequest("PUT", "/nodes/node-1",
        bytes.NewBufferString(`{"spec":{"value":"updated"}}`))
    request.Header.Set("If-Match", currentNodeETag(t))
    updated := httptest.NewRecorder()
    done := make(chan struct{})
    go func() { UpdateNode(updated, request); close(done) }()

    select {
    case <-backend.committed:
    case <-time.After(3 * time.Second):
        t.Fatal("update did not commit")
    }
    deleted := httptest.NewRecorder()
    DeleteNode(deleted, httptest.NewRequest("DELETE", "/nodes/node-1", nil))
    close(backend.resume)
    <-done

    if deleted.Code != http.StatusOK || updated.Code != http.StatusOK {
        t.Fatalf("delete=%d update=%d", deleted.Code, updated.Code)
    }
    if _, err := storage.LoadNode(context.Background(), "node-1");
        !errors.Is(err, fabricaStorage.ErrNotFound) {
        t.Fatalf("resource still exists: %v", err)
    }
    snapshots, err := storage.ListNodeVersions(context.Background(), "node-1")
    if err != nil { t.Fatal(err) }
    if len(snapshots) != 0 {
        t.Fatalf("deleted resource has %d late snapshots", len(snapshots))
    }
}
```

## Actual behavior

The spec update and resource DELETE both return `200 OK`. Loading the resource returns `ErrNotFound`, but listing its versions returns the snapshot written after deletion. Snapshot bookkeeping logs:

```text
Warning: failed to persist version into status for Node node-1: resource not found
```

The remaining snapshot can also be retrieved through the generated version endpoints, which do not require the parent resource to exist.

## Expected behavior

After resource DELETE completes, an already-pending spec mutation must not recreate snapshots for that deleted resource. The resource stays absent and its snapshot collection stays empty.

## Root cause

- [The update handler creates its snapshot after committing the resource mutation](https://github.com/OpenCHAMI/fabrica/blob/abf0a5776c6cb3a7c21ca0d5f1eaa0926b38ae94/pkg/codegen/templates/server/handlers.go.tmpl#L331-L343).
- [Atomic resource deletion cleans the version directory after its resource mutation returns](https://github.com/OpenCHAMI/fabrica/blob/abf0a5776c6cb3a7c21ca0d5f1eaa0926b38ae94/pkg/codegen/templates/storage/file.go.tmpl#L307-L321).
- [Snapshot creation takes the snapshot guard but unconditionally creates the directory and file](https://github.com/OpenCHAMI/fabrica/blob/abf0a5776c6cb3a7c21ca0d5f1eaa0926b38ae94/pkg/codegen/templates/storage/file.go.tmpl#L553-L564).

The guard serializes each filesystem operation but permits a late snapshot write after cleanup. Rejecting stale bookkeeping afterward does not remove that file.

## Suggested fix and acceptance criteria

Coordinate parent-resource existence checks and snapshot creation under the same guard used for snapshot cleanup. Check lock ordering so storage reads remain safe. If resource deletion has already committed, skip or reject the pending snapshot write.

- [ ] A deterministic regression covers a spec PUT paused after commit, a completed DELETE, and the resumed snapshot writer.
- [ ] Cover spec PATCH and the opposite snapshot-writer/cleanup ordering as well.
- [ ] Completed deletion leaves no resource and no snapshots, with no resource resurrection.
- [ ] Preserve ordinary snapshot creation and conditional snapshot deletion.
- [ ] Change generator templates and regenerate test services rather than hand-editing generated service outputs.

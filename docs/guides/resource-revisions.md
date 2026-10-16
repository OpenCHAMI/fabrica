<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Immutable resource revisions

Resource revisioning is an opt-in contract for stable named series and immutable content revisions. It is separate from API hub/spoke versioning: API versions describe wire-schema evolution, while resource revisions preserve successive values of one named resource.

## Enable revisioning

Enable each resource explicitly in `apis.yaml`:

```yaml
groups:
  - name: infra.example.io
    storageVersion: v1
    versions: [v1]
    resources: [BootConfig]
    revisioning:
      BootConfig:
        enabled: true
        bareNameSelector: default
```

Resources not listed under `revisioning`, or configured with `enabled: false`, retain the existing CRUD routes and generated artifacts. `bareNameSelector` currently accepts only `default`.

> [!IMPORTANT]
> Revision-enabled generation currently requires file storage. Fabrica rejects revision-enabled Ent generation rather than emitting a service with an incomplete storage contract.

Initialize generated services with `storage.InitFileBackend(dataDir)`. Generic `storage.Init` does not provide the data-directory boundary needed by the revision index.

### File-storage concurrency and durability

All `FileStore` instances in one process that resolve to the same canonical revision directory share a mutex. This serializes series creation, revision-number allocation, named ensure, status updates, and default promotion even when application code reopens the store. It does not provide an inter-process file lock: do not run multiple service processes against the same file-storage directory.

Series state contains the authoritative ordered revision UID list. Revision listing and lookup use that list rather than scanning record files, so a record written immediately before a failed series-state update is an ignored orphan and cannot claim a revision number or become resolvable. Orphan files are retained; this release does not run automatic garbage collection.

Each JSON file is replaced atomically with a temporary-file rename, but a revision mutation spans a record file and a series-state file. The file backend does not provide a filesystem transaction or call `fsync` across both files and their directories. A process crash between the two renames can leave an ignored orphan. Sudden host or storage-device failure can still require restoring the data directory from backup.

## Series and revisions

A series has a stable name, monotonically increasing revision numbers, and two pointers:

- `default`: the explicitly promoted revision. A bare series name resolves here.
- `latest`: the most recently created revision.

Creating a series creates revision 1 and initializes both pointers. Creating another revision advances only `latest`. Revision UIDs use the `rev_` TypeID prefix with a UUIDv7 payload; ordinary resource metadata UIDs retain their existing format.

Revision content includes API identity, user-managed metadata, and spec. Status, timestamps, and server-managed UIDs are excluded from the canonical digest. Status is mutable series-level state and does not create a new revision.

## HTTP operations

For a revision-enabled `BootConfig` at `/boot-configs`, generated routes are:

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/boot-configs?revisionName=release-a` | Create a series and first revision |
| `GET` | `/boot-configs/{name}` | Read series pointers and ETag |
| `POST` | `/boot-configs/{name}/revisions` | Create an unnamed revision |
| `GET` | `/boot-configs/{name}/revisions` | List immutable revisions |
| `PUT` | `/boot-configs/{name}/revisions/{revisionName}` | Idempotently ensure a named revision |
| `GET` | `/boot-configs/{name}/revisions/by-uid/{revisionUID}` | Read an immutable revision |
| `DELETE` | `/boot-configs/{name}/revisions/by-uid/{revisionUID}` | Retire an unreferenced revision |
| `GET` | `/boot-configs/{name}/resolve` | Resolve a reference |
| `PUT` | `/boot-configs/{name}/default` | Promote `default` with `If-Match` |
| `PUT/PATCH` | `/boot-configs/{name}/status` | Update mutable series status |

Named ensure returns `201 Created` for a new binding and `200 OK` when the same `revisionName` already has identical canonical content. Reusing the name with different content returns `409 Conflict`; a name never rebinds to a different UID.

Resolve accepts one of these query forms:

```text
/boot-configs/production/resolve
/boot-configs/production/resolve?selector=default
/boot-configs/production/resolve?selector=latest
/boot-configs/production/resolve?revisionName=release-a
/boot-configs/production/resolve?uid=rev_...
```

The first form is exactly equivalent to `selector=default`. Supplying more than one of `uid`, `revisionName`, or `selector` is invalid.

## Promote default safely

Read the series to obtain its `ETag`, then promote a revision:

```bash
etag=$(curl -fsSI http://localhost:8080/boot-configs/production | awk -F': ' 'tolower($1)=="etag" {print $2}' | tr -d '\r')

curl -X PUT http://localhost:8080/boot-configs/production/default \
  -H 'Content-Type: application/json' \
  -H "If-Match: ${etag}" \
  -d '{"revisionUid":"rev_01..."}'
```

Missing or stale `If-Match` returns `412 Precondition Failed` and leaves `default` unchanged. Creating revisions changes the series ETag because it advances `latest`.

## Generated clients and CLI

The Go client generates `CreateBootConfigSeries`, `EnsureBootConfigRevision`, `ListBootConfigRevisions`, `GetBootConfigRevision`, `RetireBootConfigRevision`, `ResolveBootConfigRevision`, and `PromoteBootConfigDefault`. Promotion requires the current series ETag.

The generated CLI groups revision operations under the resource:

```bash
cat boot-config.json | client bootconfig revisions ensure production release-a
client bootconfig revisions series production
client bootconfig revisions list production
client bootconfig revisions retire production rev_01...
client bootconfig revisions resolve production --selector latest
client bootconfig revisions promote-default production rev_01... --if-match '"sha256:..."'
```

`revisions series` prints the series status and current ETag. `revisions get` accepts a series name and revision UID. For `promote-default`, `--if-match` may be supplied explicitly; when omitted, the CLI first reads the series state and uses its returned ETag for the promotion request. A concurrent mutation between those calls still produces `412 Precondition Failed` rather than overwriting the newer state.

Generated OpenAPI, authorization tuples, and lifecycle events use the same revision routes. Promotion has a distinct `promote-default` authorization action; it is not classified as an ordinary update. Events distinguish `series-created`, `revision-created`, and `alias-promoted`, including the series, revision UID, alias, previous UID, and new UID where applicable.

## Retention

Revision hard-delete routes are not generated. `DELETE /<resources>/{name}/revisions/by-uid/{revisionUID}` retires an unreferenced revision by recording a tombstone. A revision selected by `default` or `latest` must be moved before retirement. Retired revision UIDs, names, and series-local numbers remain permanently reserved, remain visible in revision lists with `retiredAt`, and cannot resolve or be rebound.

Series retirement and deletion are intentionally unsupported in this release because there is no series tombstone or reference-safety contract. Fabrica therefore generates no series retire/delete handler, route, authorization classification, or starter policy tuple. This is an explicit API boundary rather than an unclassified protected route.

## Backup and restore

Back up and restore the complete file-storage data directory for revision-enabled resources. Copying only resource records omits series pointers, revision names, retirement tombstones, and the authoritative revision order.

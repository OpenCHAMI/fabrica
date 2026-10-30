<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Migrating resource UIDs to TypeID

New resource identities use `<prefix>_<26 lowercase base32 characters>` with
a UUIDv7 payload. Existing legacy `prefix-hex` identifiers remain valid for
reads and are never converted. Resource envelopes, list responses, and UID
field types are unchanged. Treat identifiers as opaque strings in clients,
events, resource references, and storage keys.

Rebuilding with the upgraded runtime changes new UID generation immediately,
including in previously generated handlers. Regeneration updates the service's
OpenAPI descriptions and examples; it is not what activates the runtime default.

## Before upgrading

1. Inventory the known services and their custom `RegisterResourcePrefix`
   calls or `resource.GetRegisteredPrefixes()` output. Check generated route
   prefixes too. Record the repository/revision, kind, prefix, compatibility,
   and any required remediation. The [repository prefix audit](../reference/uid-prefix-compatibility.md)
   does not substitute for this external service inventory.
2. New generation requires a nonempty prefix of at most 63 ASCII lowercase
   letters or internal underscores, beginning and ending with a letter.
   Legacy lowercase alphanumeric registrations still load for reads and kind
   lookup, but numeric and overlong prefixes cannot generate new default IDs.
   Do not silently rename those prefixes: old UID-to-kind lookup can depend on
   them. Plan explicit compatibility handling in the affected service first.
3. Update custom validators, URL construction, logging, UI display, and scripts
   that split identifiers on a hyphen or assume eight hexadecimal characters.
   Do not restrict read validation to UUIDv7: TypeID parsing accepts the other
   payloads required by specification 0.3.0, and legacy parsing remains permissive.
4. Check custom database columns and indexes for the maximum new UID length of
   90 characters. Apply the same check to reference columns. Preserve support
   for any longer historical identifiers rather than shrinking existing limits.
5. Back up persisted data and record legacy identifiers, file names/keys, and
   representative links before upgrading. Plan a rollback to a runtime that
   understands both formats, and test that choice against mixed-format data.

Use the Fabrica codec for prefix checks rather than maintaining another regex:

```go
for kind, prefix := range resource.GetRegisteredPrefixes() {
    if prefix == "" {
        return fmt.Errorf("%s has an empty UID prefix", kind)
    }
    if err := uid.ValidatePrefix(prefix); err != nil {
        return fmt.Errorf("%s prefix %q requires remediation: %w", kind, prefix, err)
    }
}
```

This snippet is a preflight to run before deployment, not an instruction to
reject legacy stored identities or rewrite the registration map at startup.

## Regenerate and validate

First update the service's Fabrica runtime dependency to the intended version
and select Go 1.26.10 while the module targets that toolchain. For local release
validation, build the generator in its checkout:

```sh
GOTOOLCHAIN=go1.26.10 go build -o ./bin/fabrica ./cmd/fabrica
```

Then, in a disposable service checkout, use that binary and runtime source:

```sh
GOTOOLCHAIN=go1.26.10 go mod edit -go=1.26.10
GOTOOLCHAIN=go1.26.10 go mod edit -replace=github.com/openchami/fabrica=/absolute/path/to/fabrica
GOTOOLCHAIN=go1.26.10 /absolute/path/to/fabrica/bin/fabrica generate --fabrica-source /absolute/path/to/fabrica
GOTOOLCHAIN=go1.26.10 go mod tidy
GOTOOLCHAIN=go1.26.10 go build ./...
GOTOOLCHAIN=go1.26.10 go test ./...
git diff --check
```

The local replacement aligns the runtime with the generator; `--fabrica-source`
alone selects generator source. Replace the local filesystem replacement with
the intended release dependency before publishing the service. Inspect the
generated diff, including routes, OpenAPI, clients, and storage. Prefix preflight
errors should appear before server output is changed. Fix source inputs and
regenerate; do not edit generated output.

Against a persistent test store, verify the following outcomes:

- A create request returns a TypeID containing UUIDv7. A client-supplied UID
  does not override the server-assigned identity.
- Legacy IDs and newly created IDs both work as generated Go client and CLI
  arguments. Lists remain bare JSON arrays.
- Updates and spec patches preserve UID and status. Status changes use the
  status subresource. Labels and annotations survive updates and reopening.
- Legacy-to-TypeID and TypeID-to-legacy references resolve using the exact
  stored strings, including after restarting the service/backend.
- File names, storage keys, and existing database rows keep their identities.
  No conversion job or data rewrite is necessary.
- Captured lifecycle/condition events preserve UID values in their source,
  extensions, and payloads. Generated handlers publish lifecycle events for
  both formats.
- Delete works for resources with labels/annotations. Ent storage deletes the
  metadata and parent resource atomically; a failed deletion retains both.

Regenerate Ent storage code to obtain the shared atomic mutation backend,
including transactional metadata cleanup. Both unconditional deletion and
ETag-guarded deletion must use that backend; do not replace it with a separate
deletion transaction when integrating migration changes. This requires no
database cascade schema migration.

## Rollback and mixed deployments

A binary from before dual-format parsing is not a safe rollback target once
TypeIDs exist in the store. Do not assume an older parser, client validator,
or auxiliary tool can handle those records even if the database can store them.
Use a tested dual-reader runtime as the compatibility baseline. A dual-reader
release that still generates legacy IDs can produce a mixed store; ensure all
participating readers and consumers handle both formats before that deployment.
These stack branches are development stages, not substitute release versions.

Do not convert TypeIDs back into short legacy IDs, truncate identifiers, rename
persisted files to a new format, or rewrite cross-resource references to make
an older binary work. If reverting data to a backup is necessary, account for
all resources and references written since that backup before taking action.

`GenerateUIDWithLength` remains an explicit deprecated legacy-format helper;
it is not an application-wide rollback switch.

## Automated coverage and limits

From the Fabrica checkout:

```sh
GOTOOLCHAIN=go1.26.10 go test -race ./pkg/resource ./pkg/uid ./pkg/storage ./pkg/events -count=1
GOTOOLCHAIN=go1.26.10 go test ./pkg/codegen -run '^TestUIDDefaultGeneratedWorkflow$' -count=1
GOTOOLCHAIN=go1.26.10 go test ./cmd/fabrica ./internal/config ./pkg/codegen -count=1
GOTOOLCHAIN=go1.26.10 go test ./pkg/resource -run '^$' -fuzz '^FuzzParseUID$' -fuzztime=10s
GOTOOLCHAIN=go1.26.10 go test ./pkg/uid -run '^$' -fuzz '^FuzzParse$' -fuzztime=10s
```

The generated workflow builds actual Go and CLI clients and exercises them
against file storage and SQLite/Ent with foreign keys enabled, with conditional
requests both disabled and enabled. It covers mixed stores, reopening,
metadata/reference preservation, lifecycle events, physical keys/rows, long
prefixes, stale conditional deletes, and transactional delete rollback. Rejected
or failed deletions must retain the resource and its metadata and publish no
deletion event. Separate tests
cover JSON/YAML resources and serialized lifecycle/condition events.

This is not evidence of PostgreSQL/MySQL runtime parity, compatibility with
every downstream validator, or the prefixes in external services. Run the same
service-level checks against the actual deployment backend and record external
audit results before declaring the migration ready for release.

The [external prefix audit](../reference/uid-prefix-compatibility.md#external-service-audit-2026-10-22)
records compatible registrations in boot-service and metadata-service at pinned
snapshots. It also identifies metadata-service's deterministic WireGuard UID
creation path, which needs separate adoption work if every new peer must use
TypeID. A compatible registered prefix alone does not update custom creation
logic or prove a service's runtime upgrade.

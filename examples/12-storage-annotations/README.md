<!--
Copyright © 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Example 12: Executable Storage Annotations

This source project generates a `User` REST API backed by Ent v0.14.5 and SQLite. It demonstrates the annotation behavior covered by Fabrica's generated-project tests without checking generated files into this directory. Before generation, every checked-in Go file belongs to a complete source-only module: `go mod tidy -diff` and `gopls check` are clean, and code that depends on generated storage lives under `testdata/` as non-Go fixtures.

## Proven capabilities

The `User` source type exercises:

- dedicated Ent storage;
- `string`, `bool`, `int`, `int64`, `float64`, `time.Time`, and `[]string` field mappings;
- typed defaults for string, bool, int, int64, and float64 pointer fields;
- a unique email constraint and portable B-tree role index;
- an immutable username;
- mutable bcrypt password storage;
- ordinary sensitive storage for a recovery hint;
- namespace, labels, annotations, and status persistence.

Field directives require dedicated Ent storage; generic Ent/file resources cannot request them, and dedicated mode with the file backend is rejected before output. Required bcrypt input is mandatory on create. On update, omission or the redacted string zero preserves the existing hash.

Resource version snapshots are file-backend only, so this Ent example rejects `+fabrica:resource-versioning=enabled` before output. Unknown Fabrica directives fail strictly with source-located typed parse errors instead of being ignored.

Sensitive dedicated fields use presence-safe update semantics. Redacted non-pointer zero values are treated as omitted and preserve storage; nonzero values replace storage. Supported non-nil pointers explicitly replace storage, including with zero; nil preserves it. Empty sensitive `[]string` values preserve storage because pointer-to-slice fields are not supported.

Generated create, PUT, and PATCH handlers reload persisted dedicated records before returning redacted responses, so defaults and immutable values are authoritative without exposing plaintext or hashes. Status endpoints update only status. File and Ent projects compile against one backend-common typed conflict contract; Ent uniqueness errors map to HTTP 409, while file storage keeps duplicate values valid.

Only bcrypt is demonstrated; bcrypt is the only supported storage transform. Encryption, Argon2, SHA-256, MySQL, GiST, and unsupported Go types are rejected for annotation-driven dedicated storage. PostgreSQL behavior is proven separately by the restricted-role integration suite, not inferred from the local SQLite run.

## Prerequisites

- Go 1.24 or newer and `gopls`;
- `curl` with `--fail-with-body`;
- `jq`;
- a C toolchain for `github.com/mattn/go-sqlite3`.

Missing prerequisites are hard failures; the scripts do not silently skip checks.

## Generation and build regression

From the Fabrica repository root:

```bash
bash examples/12-storage-annotations/verify-example.sh
```

The harness first proves the checked-in source module is tidy and passes `gopls`. It then builds the local `fabrica` binary, copies the example to a temporary directory, adds a temporary local module replacement, generates the complete project, materializes the generated-dependent server and storage-verifier fixtures, runs `go mod tidy -diff`, runs Ent code generation, tests and builds every generated package, and runs `gopls check` over the generated Go source. Finally, it starts the generated SQLite server and executes the complete live API contract. The temporary directory is removed on success or failure.

The current CLI has no `inspect`, `validate`, or `generate --dry-run --json` commands. The harness reports those capability checks as `N/A` instead of pretending they ran; generation itself performs source/config validation before output.

## Run the generated SQLite API

This block is copy/paste executable from the repository root and keeps generated output outside the checkout:

```bash
set -euo pipefail
REPO_ROOT="$PWD"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/fabrica-example-12-run.XXXXXX")"
PROJECT_DIR="$WORK_DIR/project"
SERVER_PID=""
cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID"
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

mkdir -p "$PROJECT_DIR"
cp -R examples/12-storage-annotations/. "$PROJECT_DIR/"
go build -o "$WORK_DIR/fabrica" ./cmd/fabrica
cd "$PROJECT_DIR"
go mod edit -replace "github.com/openchami/fabrica=$REPO_ROOT"
"$WORK_DIR/fabrica" generate --force --fabrica-source "$REPO_ROOT"
bash ./materialize-generated-fixtures.sh
go mod tidy
go generate ./internal/storage/ent
go test -count=1 ./...
go build -o "$WORK_DIR/server" ./cmd/server/

PORT=18080
DATABASE_URL="file:$WORK_DIR/example.db?_fk=1"
"$WORK_DIR/server" -host 127.0.0.1 -port "$PORT" -database-url "$DATABASE_URL" \
  >"$WORK_DIR/server.log" 2>&1 &
SERVER_PID=$!

for _ in {1..100}; do
  if curl --fail-with-body --silent "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; then
    break
  fi
  kill -0 "$SERVER_PID"
  sleep 0.1
done

BASE_URL="http://127.0.0.1:$PORT" \
PROJECT_DIR="$PROJECT_DIR" \
DATABASE_URL="$DATABASE_URL" \
bash ./test-api.sh
```

The server binds to loopback, not a public interface. Startup is bounded, and the trap terminates the process and removes the temporary database.

## Executable API contract

`test-api.sh` verifies these observed contracts:

- routes are `/users` and `/users/{uid}`; there is no `/api/v1` prefix;
- list returns a flat JSON array, never `{ "items": [...] }`;
- create is HTTP 201, reads/lists/updates/status/deletes are HTTP 200;
- malformed JSON is HTTP 400, a missing resource is HTTP 404, and unique conflicts on create or update are HTTP 409 with `{ "error": "storage conflict", "code": 409 }`;
- defaults are present in the dedicated create response after persisted reload; `test-api.sh` records this as `create_persisted_defaults`;
- API responses retain sensitive keys with empty-string values; they are zeroed rather than omitted;
- database verification separately proves bcrypt hashes match the create and update plaintext while never printing the hash;
- ordinary sensitive data is stored but zeroed at the API boundary;
- immutable behavior is asserted after a follow-up read, not from the update echo;
- delete returns a message object and the following read returns 404.

API redaction and database hashing are different guarantees: redaction controls JSON output, while bcrypt controls the persisted password value. After generation, `materialize-generated-fixtures.sh` creates the `cmd/verify-storage` helper from `testdata/verify-storage-main.go.txt`; it checks storage without emitting plaintext or hash material.

## Migration and cutover

Generation emits dedicated migration helpers but never calls them automatically. The migration is explicit and non-destructive: it copies eligible generic records to dedicated storage and does not delete the generic source rows. A continuation cursor advances only after the batch transaction commits; rollback returns the input cursor and zero copied rows, so retries resume from the last durable boundary. Before cutover, operators must back up the database, preview conflicts, run the helper under the intended application role, verify data and application reads, and switch traffic deliberately. Source-row retention or deletion is a separate operator-controlled action.

## PostgreSQL integration

PostgreSQL is covered by the repository's real restricted-role integration job, not by this local SQLite example. With the service configured as in [`.github/workflows/regression-tests.yml`](../../.github/workflows/regression-tests.yml), run:

```bash
FABRICA_TEST_POSTGRES_DSN='postgres://fabrica_app:password@127.0.0.1:5432/fabrica_acceptance?sslmode=disable' \
go test -tags=integration -count=1 -v ./pkg/codegen -run '^TestGeneratedPostgres'
```

That suite owns PostgreSQL-specific migrations, restricted-role behavior, and dialect-specific index assertions.

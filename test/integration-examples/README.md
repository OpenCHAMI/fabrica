<!--
Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Integration tests for examples

Each subdirectory generates a service from one of the [examples](../../examples),
runs it in Docker with [fabrun](../../fabrun), and tests it over HTTP with Go
tests written with [testify](https://github.com/stretchr/testify).

| Directory | Example |
| --- | --- |
| [basic-crud](basic-crud) | [01-basic-crud](../../examples/01-basic-crud) |
| [core](core) | Node ([03-fru-service](../../examples/03-fru-service) `FRUSpec`) and User ([12-storage-annotations](../../examples/12-storage-annotations) `UserSpec`, without annotations), generated with file, sqlite, and postgres storage and tested with one shared suite |

## Requirements

- Go, GNU make, and a POSIX shell (Linux, macOS, or Cygwin/MSYS on Windows)
- Docker with `docker compose` (used by fabrun to run the service)

## Running

From the fabrica repository root, run every example:

```sh
make test-integration-examples
```

Or run one example, either all at once or step by step:

```sh
cd test/integration-examples/storage-annotations
make test        # generate + up + test-only + down

make generate    # fabrica init, fabrica add resources resource-defs/, fabrica generate, go build
make up          # build the linux server binary and `fabrun start` it
make test-only   # run the Go tests against the running service
make down        # `fabrun stop` (removes the data volume)
make clean       # down + remove build/
```

`make up` and `make test-only` can be repeated against the same generated
service; tests create uniquely named resources so they can be re-run while the
service is up.

Useful variables: `PORT` (default `18080`), `DB` (`sqlite` or `postgres`),
`NAME` (fabrun container name), `TEST_FLAGS` (default `-count=1 -v`, e.g.
`TEST_FLAGS='-count=1 -v -run TestStorageAnnotations/TestCreateUser'`), and
`BUILD_IN_DOCKER=0` to build the server with the host toolchain on linux.

Running `go test ./...` in this directory without `FABRICA_EXAMPLE_URL` set
skips the tests.

### core

[core](core) generates the same service three times,
into `build/<service>-<backend>/`, and runs one suite against each. Every step exists for
all backends and per backend (`file`, `sqlite`, `postgres`):

```sh
cd test/integration-examples/core
make test                 # test-file, test-sqlite, test-postgres (continues past failures)
make test-sqlite          # generate-sqlite + up-sqlite + test-only-sqlite + down-sqlite

make generate             # or generate-file / generate-sqlite / generate-postgres
make up                   # or up-<backend>; all three can run at once
make test-only            # or test-only-<backend>
make down                 # or down-<backend>
```

Ports: `PORT_file` (18082), `PORT_sqlite` (18083), `PORT_postgres` (18084), and
`PG_PORT` (15433) for the postgres database container. The sqlite server needs
cgo, so it is built in a `golang:<ver>-bookworm` container unless
`BUILD_IN_DOCKER=0`. The suite is grouped into service, CRUD, patch, status,
validation, and conditional-request (ETag/If-Match) tests; see the package doc
in `suite_test.go`.

## Adding an example

Create `<example>/` with a `resource-defs/` directory of annotated `<Name>Spec`
files, a `Makefile` providing the `generate`, `up`, `test-only`, `down`, and
`test` targets (copy `storage-annotations/Makefile`), and a `_test.go` file. The
root `make test-integration-examples` target picks up any subdirectory that has
a `Makefile`.

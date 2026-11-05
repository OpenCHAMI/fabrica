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

## Adding an example

Create `<example>/` with a `resource-defs/` directory of annotated `<Name>Spec`
files, a `Makefile` providing the `generate`, `up`, `test-only`, `down`, and
`test` targets (copy `storage-annotations/Makefile`), and a `_test.go` file. The
root `make test-integration-examples` target picks up any subdirectory that has
a `Makefile`.

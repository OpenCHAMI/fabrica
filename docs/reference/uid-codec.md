<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# UID codec

`github.com/openchami/fabrica/pkg/uid` is the shared format boundary for the
UID migration tracked in [issue #85](https://github.com/OpenCHAMI/fabrica/issues/85).
Adding this package does not change the existing `resource` helpers or
revision UIDs. Ordinary resources still receive legacy IDs in this stage.

```go
var codec uid.Codec // zero value is ready for concurrent use
value, err := codec.Generate("dev")
// Handle err before using value.
parsed, err := codec.Parse(value)
// parsed contains Format, Prefix, Suffix, and the decoded UUID string.
```

Generation produces TypeID specification 0.3.0 strings with RFC 9562 UUIDv7
payloads. `ValidatePrefix` checks the specification's rules: up to 63 ASCII
lowercase letters or underscores, with no leading or trailing underscore.
The specification permits an empty prefix; resource registration can impose
an additional nonempty requirement. Internal and consecutive underscores are
allowed. Digits, uppercase letters, and non-ASCII letters are rejected.

Parsing accepts spec-valid TypeIDs, including bare suffixes with no prefix,
nil UUIDs, and non-v7 payloads required by the official conformance vectors.
The decoded UUID is a canonical string, so the public API does not expose
the TypeID dependency's types.

For legacy strings, parsing preserves `resource.ParseUID` behavior: exactly
one hyphen with a hex-decodable suffix, without checking the prefix or
requiring either part to be nonempty. This includes historical permissive
cases such as `dev-` and `-`; acceptance is not a recommendation to generate
them. Uppercase hexadecimal is preserved. A legacy result has an empty
`UUID` field. Parsing never changes stored IDs, checks registration, or logs
deprecation messages.

`NewCodec(generator)` accepts an instance-local function returning
`([16]byte, error)` for deterministic tests. Nil selects the default
generator. Custom generators must produce UUIDv7 with the RFC 9562 variant;
the codec rejects other values and wraps source errors. Prefix validation
precedes generator invocation. Callers sharing a custom codec concurrently
must supply a concurrency-safe generator. No global test hooks are used.

## Dependency policy

The TypeID implementation is pinned to `go.jetify.com/typeid/v2` at
`v2.0.0-alpha.3`, the latest published v2 prerelease checked for this stage.
It implements prefix validation, encoding, and parsing behind `pkg/uid`.
Generation uses the existing `github.com/google/uuid` dependency for UUIDv7;
Jetify also brings `github.com/gofrs/uuid/v5` transitively. The prerelease
dependency is deliberate: upgrades require review, passing the vendored
0.3.0 vectors and codec tests, and verification that public Fabrica behavior
remains stable. Generated services should consume Fabrica's API rather than
import the TypeID implementation directly.

Run the focused checks under the repository's selected toolchain:

```sh
GOTOOLCHAIN=go1.26.10 go test -race ./pkg/uid -count=1
GOTOOLCHAIN=go1.26.10 go test ./pkg/uid -run '^$' -fuzz '^FuzzParse$' -fuzztime=10s
```

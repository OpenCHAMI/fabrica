<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# UID prefix compatibility

This audit supports the default TypeID migration in
[issue #85](https://github.com/OpenCHAMI/fabrica/issues/85). New resource UIDs
use TypeID/UUIDv7. Numeric legacy prefixes still register for reads and kind
lookup, but cannot generate new UIDs. TypeID-compatible nonempty prefixes,
including internal underscores, also register. Existing identifiers are never
rewritten. The explicit `GenerateUIDWithLength` legacy helper is deprecated.

## Repository audit

The audit covers tracked literal `RegisterResourcePrefix` calls and examples,
plus resource envelopes in the checked-in examples and integration fixtures,
as of the codec foundation commit `102d600`. Generated and ignored local
projects are outside this audit. The new tests deliberately add a numeric
prefix to prove that legacy registration remains compatible.

| Source | Kinds and registered prefixes | Result |
| --- | --- | --- |
| Resource package documentation | Device/dev, Asset/ast, Sensor/sen | Compatible |
| Resource model guide | Device/dev, User/usr, Product/prd | Compatible |
| Rack integration fixtures and tests | RackTemplate/rktmpl, Rack/rack, Chassis/chas, Blade/blade, BMC/bmc, Node/node | Compatible |

Resource envelopes found in examples and integration fixtures are BMC,
Blade, Chassis, FRU, Node, NodeSet, ProfileBinding, Rack, RackTemplate, and
Token. All their lowercase full names are TypeID-compatible. Their legacy
CLI-derived three-byte prefixes are also compatible with the TypeID grammar.

The two generation paths need separate treatment when changing the default:

| Path | Current prefix derivation | Migration concern |
| --- | --- | --- |
| `pkg/codegen/templates/server/routes.go.tmpl` | Entire lowercase resource name | Names containing digits, disallowed underscores, non-ASCII characters, or more than 63 bytes can be incompatible. |
| Legacy resource definitions from `cmd/fabrica/add.go` | First three bytes of the lowercase resource name | Short names use their full lowercase name. Both full and shortened prefixes are validated before writing. Several kinds can share the same first three bytes. |

Do not silently rename or truncate incompatible prefixes. A prefix change
affects kind lookup for existing identifiers. The scaffolder and server-generation paths validate derived prefixes before
writing output. Existing numeric prefixes require explicit remediation before
new TypeID creation can succeed; accepting their legacy identifiers remains
supported. There is no automatic fallback to generating legacy IDs.

Recheck literal registrations and prefix derivation sites with:

```sh
rg -n 'RegisterResourcePrefix\(' pkg cmd examples test docs
```

## Generator preflight

After discovering resources, callers of `pkg/codegen` can run:

```go
if err := generator.ValidateTypeIDPrefixes(); err != nil {
    return err
}
```

This method validates the lowercase names used by the routes template
and reports all incompatible kinds and prefixes without renaming them.
`GenerateAll` for servers, direct `GenerateRoutes`, and the CLI server runner
enforce it before emitting server files. CLI discovery also validates before
writing resource registration output. Empty prefixes are
rejected even though the TypeID specification permits unprefixed IDs.
It does not detect registration collisions or inspect custom registrations,
including the legacy CLI's resource-file registration. This is a Go API
preflight, not a new CLI flag.

Tests generate routes and inspect the emitted registrations to prove that
the preflight checks the same prefixes. A separate test proves that a
numeric resource name fails server generation without modifying output.

## External service audit remains open

No external generated service repositories or deployed prefix registries
were available for this audit. Before the default change, collect each
known service's custom registrations (or `resource.GetRegisteredPrefixes()`
output) and validate their prefixes with `uid.ValidatePrefix`, also requiring
a nonempty prefix. Record incompatible prefixes and their remediation.
The repository audit alone does not complete that issue acceptance criterion.

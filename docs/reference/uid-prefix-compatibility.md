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
preflight, not a new CLI flag. Those custom registrations are covered by the
startup warning below instead.

Tests generate routes and inspect the emitted registrations to prove that
the preflight checks the same prefixes. A separate test proves that a
numeric resource name fails server generation without modifying output.

## Startup warning in generated services

The generator preflight only sees prefixes derived from resource names. Generated
services therefore also audit the live registry at startup: the regenerated
`registerResourcePrefixes()` in `routes_generated.go` calls
`warnIncompatibleUIDPrefixes()`, which walks `resource.GetRegisteredPrefixes()`
and logs one warning per prefix that `uid.ValidatePrefix` rejects. This covers
prefixes registered by hand in resource files, which the generator cannot see.
Because the call lives in generated code, regenerating picks it up without
editing user-owned `cmd/server/main.go`.

The check warns and the service still starts. Legacy prefixes stay registered so
stored identifiers keep resolving to their kind, and refusing to start would
remove the read access this migration preserves. Creating a resource of the
affected kind still fails with HTTP 500 until the prefix is remediated.

```text
WARNING: resource kind "Node" prefix "node2" cannot generate new TypeID UIDs:
uid: invalid TypeID prefix: typeid: prefix must contain only [a-z_], found '2'
in "node2". Reads and kind lookup still work, but creating a Node fails with
HTTP 500 until the prefix is remediated.
```

Prefixes registered after `registerResourcePrefixes()` runs are not reported.
Renaming a prefix changes kind lookup for existing identifiers, so there is no
in-place remediation today; an affected service needs explicit compatibility
handling before new TypeID creation can succeed. Tests assert that generated
routes call the audit and that a service with no resources emits neither the
helper nor its imports.

## External service audit: 2026-10-22

The two services below were inspected at pinned default-branch snapshots.
Every Go file was parsed to locate `RegisterResourcePrefix` calls; all seven
registrations were string literals. Their kinds agree with `apis.yaml`, and
their prefixes agree with the lowercase names used by the current routes
generator. Each nonempty prefix passed Fabrica's `uid.ValidatePrefix` and a
shared-codec TypeID generation/parsing round trip under Go 1.26.10.

| Service snapshot | Registered kinds and prefixes | Prefix result |
| --- | --- | --- |
| [boot-service, `1934967`](https://github.com/OpenCHAMI/boot-service/blob/1934967ada3c9ec722e434c6930835c8a29a0e0f/cmd/server/routes_generated.go#L84-L88) | BMC/bmc, BootConfiguration/bootconfiguration, Node/node | All compatible; no prefix renaming required |
| [metadata-service, `fed117c`](https://github.com/OpenCHAMI/metadata-service/blob/fed117cde0ad7ce680b63dcdc90ca89b8641f185/cmd/server/routes_generated.go#L48-L54) | ClusterDefaults/clusterdefaults, Group/group, InstanceInfo/instanceinfo, WireGuardPeer/wireguardpeer | All compatible; custom creation exception below |

These snapshots use Fabrica v0.4.13 with Go 1.26.10 in boot-service and
Fabrica v0.4.10 with Go 1.26.5 in metadata-service. Both services need their
runtime dependency upgraded to obtain the new default; metadata-service also
needs its selected toolchain/module requirements aligned with Go 1.26.10.
Generated create handlers call `GenerateUIDForResource`, so those creation
paths pick up the new format after the runtime upgrade.

### Metadata-service custom WireGuard identities

The custom WireGuard initialization route bypasses Fabrica's generation
helper. [`wireGuardPeerUIDFromPublicKey`](https://github.com/OpenCHAMI/metadata-service/blob/fed117cde0ad7ce680b63dcdc90ca89b8641f185/cmd/server/wireguard_routes.go#L218-L224)
returns `wireguardpeer-` plus 16 hexadecimal characters derived from the public
key. The [initialization handler](https://github.com/OpenCHAMI/metadata-service/blob/fed117cde0ad7ce680b63dcdc90ca89b8641f185/cmd/server/wireguard_routes.go#L81-L115)
uses that value for both existing-peer lookup and new resource metadata. Its
idempotency therefore depends on the deterministic identity. A Fabrica runtime
upgrade alone leaves this path generating legacy IDs, even while the generated
WireGuardPeer CRUD handler starts generating TypeIDs.

The existing 16-character suffix remains valid legacy input. Preserve those
identities and their references. If all newly created peers must use TypeID,
make a separate metadata-service change that finds existing peers by a stable
public-key identity and assigns TypeID only when creating a new peer. Preserve
idempotency and concurrency behavior; replacing the deterministic helper with
a fresh random UID on every request would lose existing-peer lookup.

### Audit scope and remaining release checks

This completes the prefix audit for these two named source snapshots. It does
not establish compatibility for additional services, deployed configuration,
or every custom endpoint. No service sources were modified, and these service
snapshots were not rebuilt or regenerated against the new runtime as part of
the prefix check. The WireGuard exception is a downstream adoption follow-up,
not an incompatible registered prefix.

Add any other known services to the inventory, collect custom registrations
or `resource.GetRegisteredPrefixes()` output, and record prefix compatibility
and remediation before release. Follow the [UID migration guide](../guides/uid-migration.md)
for service-level upgrade and mixed-store validation.

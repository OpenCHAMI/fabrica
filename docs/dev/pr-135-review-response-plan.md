<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# PR #135 review response plan

Status: step 1 implemented and verified. Steps 2–4 remain proposed.

Reviewed on 2026-10-27 against PR head `e947660` and its current checkout.
All eight inline comments across five unresolved review threads were retrieved,
including replies, along with the complete review summary and manual test steps.

PR: <https://github.com/OpenCHAMI/fabrica/pull/135>

## Assessment and scope

The three initial loader defects and the build-tag follow-up are present in the
current PR. The reviewer supplied four commits with implementation, tests, and
documentation. Reuse those changes after inspecting and replaying them onto the
current PR base. The validation and empty-Ent-collection findings also match the
current source, but both predate the loader. Treat their scope explicitly.

Retain the existing compatibility boundary: new/adopted services can change
supported generation settings without main edits; legacy services need the
documented one-time migration. Regeneration requires rebuilding/restarting the
service and does not migrate stored data or reload a running server.

## Comment coverage

| Work item | Review evidence | Proposed disposition |
| --- | --- | --- |
| Ent-to-file transition leaves incompatible Ent support files | [Storage transition comment](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4354513669) | Replay `ddb566f`; reject before writes and require preservation/migration of obsolete files. |
| `Generator.GenerateAll` bypasses feature composition and ownership preflight | [Go API comment](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4354523823) | Replay `3ab3698`; use the feature path for server generation and retain legacy fallback. |
| Loader adoption fails when application declarations move out of main | [Package-scanning comment](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4354544542) | Replay `b621e4e`; inspect active application files rather than main alone. |
| Adoption ignores the invoking build tags | [Build-tag follow-up](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4364088807) | Replay `9a959d5`; honor GOFLAGS tags through a private build context. |
| Validation modes change files but not request acceptance | [Validation contract](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4363011277), [edge cases](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4363018045) | Define and implement an effective validation policy in a separate, reviewable change; preserve mandatory request checks and strict defaults. |
| Empty Ent lists return JSON null | [Backend contract](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4363952424), [edge cases](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4363954591) | Plan a separate backend correction and tests; distinguish it from a new loader regression or a data migration. |

## 1. Replay and verify the supplied fixes

The review's fast-forward suggestion was made against an earlier branch base.
GitHub currently reports the supplied branch tip as diverged from this PR head.
It contains earlier versions of our two original feature commits, followed by
these four fixes:

1. `ddb566f4b4ce9a55f52ff3b1a8be8dac6a5a982f` — guard Ent-to-file migrations.
2. `3ab36989ec918510694feaa96a7495b70b79c724` — integrate `GenerateAll`.
3. `b621e4e4691a16f2e80b0719c2a131d98041f759` — detect adoption across application files.
4. `9a959d5575cb6d5735f28ec1763f6ef048601e2b` — honor GOFLAGS build tags.

At implementation time, fetch the proposed commits over HTTPS and replay only
these four, in order, preserving authorship. Do not replace the branch with the
older proposed tip or replay the duplicate feature commits. Retain the merged
API catalog from #137 and the atomic mutation behavior from #133.

Verify the following contracts before considering these comments addressed:

- **Storage preflight:** An Ent-to-file transition must reject before changing
  registration, handlers, storage, helpers, or the manifest if obsolete generic
  or dedicated adapters, Ent directories, query/transaction helpers, generation
  support, or export/import commands remain. Include edited/unowned files and
  the paths reported by the reviewer's guard. The diagnostic must list what to
  preserve outside the service. Never silently delete those files. After explicit
  preservation/migration, generate, tidy, test, and build the file-backed service.
- **Go API parity:** `GenerateAll` checks feature ownership and transitions before
  server writes, then composes startup through `GenerateFeatures`. Metrics/events
  toggles must keep composition and the manifest synchronized. Disabled metrics
  must not introduce an untracked compatibility stub for an adopted service.
  An edited output must leave all other generated files unchanged on rejection.
  Legacy, client, and non-server paths retain their existing behavior.
- **Package adoption:** Config and startup declarations can be split between
  user-owned application files and remain unchanged across regeneration. Tests,
  generated helpers, other packages, and inactive build variants cannot supply
  adoption evidence. Malformed active source must produce an error. Preserve
  support for migrated main files that still carry an old generated header.
- **Build context:** The generator and service build must use the same GOFLAGS
  environment. Cover both markers being tagged, only Config being tagged,
  inactive alternatives, quoting, comma/space-separated tag lists, repeated flags,
  empty resets, and malformed arguments. Do not mutate `build.Default`. Check
  normal GOOS/GOARCH filename/build-constraint selection alongside custom tags.

The supplied commits already add many of these regressions. Run and inspect them
first; add coverage for any gaps rather than duplicating equivalent tests.

## 2. Define validation policy before changing handler behavior

Recommended scope: honor the documented strict/warn/disabled modes for optional
Spec/Status struct-tag checks and framework custom-validation hooks in generated
handlers. Keep JSON decoding, endpoint-required envelope/identity checks, patch
structure/path restrictions, authorization, ETag preconditions, immutable fields,
and persistence constraints mandatory. Do not implement this by skipping every
`ValidateResource` or `ValidateWithContext` call indiscriminately.

This is a proposed behavior decision, not an already agreed reviewer requirement.
Implement it as a distinct change so it can be reviewed independently. If this
behavior expansion is deferred from #135, explicitly narrow the documented loader
contract and retain this work as a follow-up; changing middleware emission alone
does not resolve the documented HTTP behavior gap.

Use one effective-policy resolver for configuration normalization and generation:

| Input | Effective policy |
| --- | --- |
| enabled=true, mode=strict or an omitted mode | Strict; validation errors reject. |
| enabled=true, mode=warn | Run optional validators, log failures, continue when mandatory checks permit. |
| enabled=true, mode=disabled | Disabled; skip optional validation. |
| enabled=false, any otherwise valid mode | Disabled; the boolean takes precedence. |
| Invalid mode | Reject configuration using the existing validation rules. |

Keep default validation strict. Generate policy from `.fabrica.yaml`; do not let a
stale service runtime setting select a different policy. Preserve the public
behavior of standalone `pkg/validation` functions unless an explicit new API is
introduced. Separate mandatory and optional checks where the current functions
bundle them together.

Implementation targets: `internal/config`, `internal/projectfeatures`,
`pkg/codegen/features.go`, template-data plumbing in `generator.go`,
`templates/server/handlers.go.tmpl`, and the validation middleware template.
Audit create, spec PUT/PATCH, status PUT/PATCH, and revision-specific
create/update/status paths so equivalent operations follow the same policy.
Explicitly classify required revision selectors and identifiers as mandatory.

Keep validation of a prospective mutation inside the atomic mutation callback
added by #133. Strict validation failures must leave storage unchanged. Custom
validators must remain safe to retry; warn-mode diagnostics may repeat on retries.
Spec endpoints must preserve status, and status endpoints must preserve spec.

Acceptance tests must compare both fresh services in each effective mode and
services regenerated through strict → warn → disabled → strict:

- A valid request succeeds in every mode.
- A payload exceeding the default Node description limit rejects in strict mode,
  logs and proceeds in warn mode, and bypasses that optional check when disabled
  on a backend that permits the value. Confirm HTTP and persisted values.
- Exercise both contradictory enabled/mode combinations in the policy table.
- Malformed JSON, missing endpoint-required identity/envelope fields, invalid
  patches, and failed storage constraints retain their applicable errors in all
  modes. Do not require identical backend errors where existing contracts differ.
- Cover create, spec PUT/PATCH, status updates, and a custom validator. Confirm
  rejection does not mutate storage or publish successful-mutation events.
- Removing optional middleware must not leave dangling imports/helper references.
  Main and user-owned files remain unchanged; strict defaults stay compatible.

Update `docs/guides/validation.md`, the loader guide, and relevant configuration
reference/examples with this boundary and precedence table. Preserve AuthZ's
existing environment-driven mode selection; YAML AuthZ settings alone must not
be presented as changing enforcement.

## 3. Separate the empty-Ent-collection backend correction

The generic Ent storage template uses a nil result slice after a successful
zero-row query, producing JSON null. Correct that successful result to a non-nil
empty collection. Audit the dedicated Ent path for the same contract, without
changing unrelated query, conversion, or error handling.

Deliver this as a separate backend fix, preferably before the final cross-backend
acceptance run. If it is outside #135's agreed scope, retain a concrete follow-up
and label the current result accurately; do not report that a backend switch
caused a new loader regression. This plan does not create an issue or another PR.

Acceptance tests:

- Fresh file/SQLite-Ent services and services transitioned to an empty target
  backend return HTTP 200 with JSON `[]` for successful empty collection queries.
- Nonempty responses remain bare arrays with intact envelopes, UID, labels,
  annotations, spec, and status.
- An uninitialized backend, failed query, and missing single resource retain their
  existing error semantics. Do not convert errors or arbitrary nil results into
  successful empty lists.
- Existing data in the source backend is preserved; no test assumes changing
  storage configuration copies data into the target backend.

## 4. Close the remaining acceptance gaps

Extend the real checkout-built CLI fixtures instead of relying only on template
string checks. Existing tests toggle metrics, auth, events, and reconciliation
together; add independent transitions so one feature cannot mask another:

- Metrics off → on → off, with route registration and implementation removal.
- Events enabled without reconciliation, then reconciliation enabled and its
  workers/periodic interval changed, then both disabled. Verify runtime behavior
  and generated interval values, plus startup/shutdown cleanup.
- Conditional requests off → on → off, SHA-256/MD5 changes, and actual ETag/304/412
  behavior. Truncated ETag length alone is not an algorithm test. Keep #133's
  conditional-write race checks.
- AuthN off → on → off, required JWKS failures, signed-token success, and public
  health/status/docs/OpenAPI/catalog/metrics access. Disable both auth flags when
  testing auth removal. Cover retained AuthZ off/shadow/enforce semantics.
- File → SQLite/Ent and explicitly migrated Ent → file, preserving the source
  dataset and user code. Keep custom storage project-owned.
- Fresh-init versus regenerated normalized configurations, stable repeated output
  and manifests, selective-transition/ownership rejection before writes, and live
  legacy behavior before and after explicit adoption.

When interpreting removed metrics routes, account for Chi's existing OPTIONS
wildcard under auth: an unregistered GET may be 405 rather than 404. Verify
registration and middleware behavior as well as response status. Restart/rebuild
after generation; a running binary is not the acceptance target.

## Validation and completion

Use Go 1.26.10 and focused tests while applying each change, then run:

```sh
GOTOOLCHAIN=go1.26.10 go test ./cmd/fabrica ./internal/config ./pkg/codegen ./internal/projectfeatures ./internal/mcp ./pkg/reconcile ./pkg/storage ./pkg/validation -count=1
GOTOOLCHAIN=go1.26.10 make build
(cd test/integration && GOTOOLCHAIN=go1.26.10 go mod tidy && GOTOOLCHAIN=go1.26.10 go test -v -timeout 10m)
git diff --check
```

Invoke the checkout-built binary by absolute path with `--fabrica-source` for
generated-service validation. Tidy when imports change, test/build the generated
services, and run the focused atomic-write tests with their race checks. Inspect
generated output and exercise documented HTTP behavior on fresh and regenerated
services; compilation alone is insufficient.

Completion evidence should map each thread to the adopted commit(s), focused test
names/results, and any explicitly deferred pre-existing behavior. Prepare
copy-paste review replies if requested. Do not post comments, resolve threads,
create follow-up issues, merge, or push without user authorization.

The initial planning pass inspected source and proposed patches without applying
commits or rerunning tests. Step 1 delivery is recorded below.


## Step 1 delivery — 2026-10-27

The four proposed fixes were replayed onto the current feature branch. Original
Git author identity, author date, and Signed-off-by trailers were verified against
the source commits. Each replay also records its original commit hash:

| Reviewer commit | Replayed commit |
| --- | --- |
| `ddb566f` | `c571539` |
| `3ab3698` | `6cd8870` |
| `b621e4e` | `461d5b6` |
| `9a959d5` | `e7fee6b` |

The only commit-message additions are cherry-pick provenance references. The
reviewer's source changes were preserved; the current main's catalog and atomic
mutation changes remain ancestors of the resulting branch.

Verification found one additional migration gap: `ent_revisions_generated.go`
was absent from the obsolete Ent path list. A regression first demonstrated that
this file alone allowed an unsafe transition. A separate follow-up adds the path,
preserves the file on rejection, and documents it in the migration instructions.
Additional tests cover GOOS/GOARCH filename selection and unchanged output when
malformed GOFLAGS is rejected through `GenerateAll`. These additions are kept
separate from the reviewer-authored commits.

Focused regressions and all eight package suites in the validation command passed
under Go 1.26.10, including tagged generated file/Ent services and migration back
to file storage. The checkout-built CLI compiled successfully. The full separate
integration suite passed under Go 1.26.10. Formatting and `git diff --check` are
clean. No changes for steps 2–4 were implemented.

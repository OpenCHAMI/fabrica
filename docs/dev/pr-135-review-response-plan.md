<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# PR #135 review response plan

Status: steps 1 and 2 implemented and verified. Steps 3–4 remain pending. The
empty-Ent-collection correction is outside PR #135 and will be tracked separately.

Initial review: 2026-10-27 against PR head `e947660`. Refreshed against
`698f3d8` on the same date: all nine inline comments across six unresolved threads
were retrieved, including Alex Lovell-Troy's new crash-recovery comment. The
original review summary and manual test steps remain part of the acceptance plan.

PR: <https://github.com/OpenCHAMI/fabrica/pull/135>

## Assessment and scope

Step 1 delivered the three initial loader fixes and the build-tag follow-up,
preserving the reviewer's authorship. Alex's new comment identifies an additional
loader defect: rendering first does not make the subsequent feature-file and
manifest updates crash-recoverable. The user selected step 2 next; that validation
policy is now implemented as a separate change. Crash recovery remains step 3,
followed by step 4's acceptance coverage.

The validation and empty-Ent-collection findings predate the loader. Validation
scope is implemented in step 2. The user has decided to split the empty Ent
collection contract gap into a different issue: its implementation and acceptance
criteria are outside #135 and are not prerequisites for this PR's completion.
Record that issue's link when available; no issue number has been supplied, and
this plan update does not create one.

Retain the existing compatibility boundary: new/adopted services can change
supported generation settings without main edits; legacy services need the
documented one-time migration. Regeneration requires rebuilding/restarting the
service and does not migrate stored data or reload a running server.

## Comment coverage

| Work item | Review evidence | Disposition |
| --- | --- | --- |
| Ent-to-file transition leaves incompatible Ent support files | [Storage transition comment](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4354513669) | Completed in `c571539`, with revision-helper coverage in `3e8d157`; reject before writes and preserve obsolete files explicitly. |
| `Generator.GenerateAll` bypasses feature composition and ownership preflight | [Go API comment](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4354523823) | Completed in `6cd8870`; server generation uses feature composition and retains legacy fallback. |
| Loader adoption fails when application declarations move out of main | [Package-scanning comment](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4354544542) | Completed in `461d5b6`; inspect active application files rather than main alone. |
| Adoption ignores the invoking build tags | [Build-tag follow-up](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4364088807) | Completed in `e7fee6b`; honor GOFLAGS tags through a private build context. |
| Validation modes change files but not request acceptance | [Validation contract](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4363011277), [edge cases](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4363018045) | Implemented in step 2: shared effective-policy resolver and generated handler policy, with mandatory request checks and strict defaults preserved. |
| Interrupted feature writes leave files and ownership manifest inconsistent | [Alex's crash-recovery comment](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4390596505) | New step 3: persist recoverable transaction intent before mutation, atomically replace files/manifest, and prove successful retry after failures. |
| Empty Ent lists return JSON null | [Backend contract](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4363952424), [edge cases](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4363954591) | User decision: separate issue, outside #135. Retain follow-up context below; do not gate loader completion on this backend correction. |

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

Implemented scope: honor the documented strict/warn/disabled modes for optional
Spec/Status struct-tag checks and framework custom-validation hooks in generated
handlers. Keep JSON decoding, endpoint-required envelope/identity checks, patch
structure/path restrictions, authorization, ETag preconditions, immutable fields,
and persistence constraints mandatory. Do not implement this by skipping every
`ValidateResource` or `ValidateWithContext` call indiscriminately.

The user selected this behavior expansion for implementation. It is a distinct
change, with shared policy APIs in `pkg/validation`, consistent configuration and
feature normalization, and policy-aware generated handlers and middleware. The
existing standalone validation APIs retain strict behavior.

Implemented regression coverage:

- Configuration/default/invalid-mode precedence and rejection before generator writes.
- Checkout-built CLI tests of fresh strict/warn/disabled services, regeneration
  strict → warn → disabled → strict, and disabled boolean with strict/warn modes.
- HTTP and storage checks for the default Node's 201-character description,
  custom hooks, spec PUT/PATCH, status PUT/PATCH, warning logs, disabled hook
  invocation, unchanged state/no success events after strict rejection, and
  preservation of spec/status and user-owned main/custom validation code.
- Revision series creation, revision creation/ensure, status PUT/PATCH, custom
  hooks, immutable revision preservation, required series names/promotion selectors,
  malformed JSON/patches, duplicate series constraints, and ETag failures.
- Existing generated-handler atomic mutation, conditional request, custom-backend,
  and file/Ent generation tests remain part of verification.

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

## 3. Make feature writes crash-recoverable

Source: [Alex's review comment](https://github.com/OpenCHAMI/fabrica/pull/135#discussion_r4390596505).
The current `GenerateFeatures` renders all enabled outputs in memory, then writes
or removes files sequentially and writes the manifest last. A failure after any
mutation can leave mixed output with old manifest digests, so the next ownership
check mistakes Fabrica's changes for user customization. The shared write helper
also uses direct `os.WriteFile`, allowing a partial manifest write.

Implement this step next. Atomic replacement of the manifest alone
is insufficient: recovery must recognize a recorded mixture of old and target
feature outputs. The guarantee applies to loader-owned outputs and their
manifest; it does not make every generator operation or storage migration one
atomic transaction.

### Transaction and recovery design

1. **Prepare exact bytes and durable intent.** After normal config, transition,
   and ownership preflight, stage the final bytes for all writes and the target
   manifest. Preserve timestamp-only no-op behavior and generated identity rules
   when choosing those bytes, so recorded digests match what will actually be
   written. Before the first output mutation, atomically persist a versioned
   journal containing the old and target manifest digests, effective generation
   configuration, and each owned path's old/target digest or explicit absence.
   Keep immutable staged data until recovery/commit completes. Flush staged data,
   journal, and directory metadata as supported; document durability guarantees.
   Preparation failures must leave live outputs unchanged.
2. **Commit individual operations atomically.** Use temporary files on the
   destination filesystem and rename to replace each changed output. Record
   removals in the journal and remove only files matching the recorded old state.
   Replace the final manifest through the same atomic-write mechanism after all
   feature outputs match the target. Do not truncate a live output or manifest.
   Serialize mutation/recovery for a service so concurrent generators cannot
   replace each other's transaction intent.
3. **Recover before ordinary ownership rejection.** Prefer idempotent roll-forward
   to the journal's captured target. For every path and the manifest, accept only
   its recorded old or target state, including expected creation/deletion states.
   Complete missing operations and commit the captured manifest, then clean up
   staging and journal state. A crash after manifest replacement but before
   cleanup must be recognized as a completed transaction on the next run.
4. **Preserve user edits and request boundaries.** A path matching neither
   recorded state remains an ownership conflict; preserve it and retain recovery
   evidence. Validate journal/staging paths against the fixed owned-output list
   and reject corrupt metadata without overwriting files. A new YAML selection
   must not be mixed into an unfinished transaction: finish the captured intent
   before planning a new full generation. Document this ordering.
5. **Keep previews read-only.** Separate transaction inspection from mutating
   recovery. Full CLI generation, `GenerateAll`, and `GenerateFeatures` must enter
   recovery before a stale manifest can trigger ordinary ownership rejection.
   Selective generation with pending recovery should reject before writes and
   direct the user to full generation. MCP dry runs inspect and report pending
   recovery, affected files, and the required next action without repairing files.

Primary targets: `pkg/codegen/features.go`, the feature-file write path currently
using `writeGeneratedFile`, direct `GenerateAll`/CLI entry points, MCP preflight
and previews, and `docs/guides/feature-loader.md`. Introduce small transaction/I/O
helpers as needed without changing unrelated generators' overwrite contracts.

### Fault-injection acceptance

Use deterministic filesystem-operation hooks and subprocess interruption tests;
permission changes alone may not produce the same failure under CI privileges.

- Reproduce the reviewer's failure: one feature output has changed, the next
  output write fails, the old manifest remains, and retry after clearing the
  failure successfully repairs the recorded transaction.
- Fail before/after each owned output write and removal, including creating a
  new file and disabling an existing feature. Cover staging/journal persistence,
  the final manifest write/rename, and cleanup. After removing the injected
  failure, retry must produce target files and a manifest with matching digests.
- Terminate a subprocess after the durable journal, between mutations, and after
  manifest commit. Recovery must be idempotent, preserve user-owned files, and
  permit a subsequent normal generation/build without manual manifest editing.
- Simulate short writes/disk-full and malformed recovery metadata. Live manifests
  must not become partially written; unknown states must retain evidence and
  return actionable errors rather than discard customization.
- Edit a pending transaction's output between attempts: do not overwrite that
  unrecognized edit. Test changed YAML, selective retry, read-only MCP previews,
  and concurrent generators against the recovery policy above.
- Verify repeated successful generation remains stable, including timestamp-only
  no-ops, and that legacy services retain their documented behavior. Existing
  no-write checks for rejected ordinary ownership/selection preflight still pass.

## Separate issue: empty Ent collections (outside #135)

The user has decided to track this pre-existing backend contract gap separately.
The generic Ent template currently returns a nil slice after a successful
zero-row query, producing JSON null. The separate issue should cover non-nil
empty collections, dedicated-backend consistency, preserved resource metadata,
and unchanged query-error/missing-resource behavior. Retain the original review
links in the coverage table and add the issue link when assigned.

This is not step 3, a prerequisite for cross-backend loader checks, or a PR #135
acceptance gate. Loader transition tests should validate successful generation,
builds, selected backend startup, data preservation, and unchanged behavior
relative to a fresh service using that same backend. Record the existing empty
Ent response as the known separate issue instead of asserting its correction in
this PR. Storage configuration changes still do not migrate data.

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
- Fault-injected and interrupted feature transactions recover on full retry,
  while selective requests and previews preserve their no-write guarantees.
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
names/results, and the external disposition of the empty-Ent-collection issue.
PR #135 completion includes step 3's crash-recovery evidence; it does not require
the separate backend response-shape fix. Prepare
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


## Review scope update — 2026-10-27

Incorporated Alex Lovell-Troy's interrupted-generation/ownership-manifest feedback
as the new step 3, with transaction persistence, atomic replacement, retry/recovery,
read-only previews, and fault-injection acceptance. Recommended execution order is
step 3 → step 2 → step 4 following the completed step 1.

Recorded the user's decision to move the pre-existing empty-Ent-collection gap to
a separate issue. Its fix is excluded from this PR and its completion criteria.
This update changes the plan only; no recovery implementation, backend changes,
GitHub comments, or new GitHub issues were created.


## Step 2 delivery — 2026-10-27

Implemented generation-time strict/warn/disabled policy for optional Spec/Status
tags and framework custom-validation hooks. The same resolver validates and
normalizes YAML configuration, the project-feature bridge, and generator settings.
Omitted YAML settings and the bridge's nil-config default remain strict; false
wins over any valid mode, while invalid modes still reject. Invalid generator
settings reject before server outputs change.

New policy APIs preserve mandatory envelope checks and leave the original
standalone validation APIs strict. Generated create, spec PUT/PATCH, status
PUT/PATCH, and revision create/ensure/status operations use the effective policy.
Promotion selectors stay mandatory. Prospective resource validation remains
inside the existing atomic mutation callback. Warn logs failures and continues;
disabled invokes neither optional tags nor custom hooks. Optional middleware
emission and generated handlers agree, including after regeneration.

Verified with Go 1.26.10:

- All eight package suites in the validation command above passed, including the
  final full `pkg/codegen` rerun, generated conditional-mutation race checks,
  fresh/regenerated HTTP policy tests, revision policy tests, and file/Ent services.
- The project-feature bridge's nil/default/invalid/precedence regressions passed.
- `make build` produced the checkout binary; the separate integration module's
  `go mod tidy` and full `go test -v -timeout 10m` passed (246 seconds).
- PR-focused golangci-lint 2.15.0 reported zero issues. Changed Go files are
  formatted and `git diff --check` is clean.

Updated the validation, feature-loader, middleware, and CLI configuration guides.
The four reviewer-authored commits remain intact. Step 3's crash recovery and the
remaining step 4 acceptance work are pending; the empty-Ent-collection correction
remains separately scoped. No GitHub comments, issues, merges, or pushes were made.

<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# AGENTS.md — Directives for AI Agents Working With Fabrica

This is the single source of truth for agents, and the only always-on instruction file in this
repo. Read it before making any change. If something here conflicts with a doc under `docs/`,
this file wins; open an issue to reconcile the doc.

`.github/copilot-instructions.md` is a two-line pointer back here — keep it that way. Do not
reintroduce a parallel summary: VS Code loads all always-on instruction files with no
guaranteed precedence, so duplicated guidance drifts with no defined winner.

---

## 1. Know which repo you are in

Fabrica is a **code generator**, not a runtime framework you edit in place. Two very
different jobs use the same tool, and the rules differ:

| Job | You are editing | Never edit |
|-----|-----------------|------------|
| **A. Working ON Fabrica** (this repo, `github.com/openchami/fabrica`) | `pkg/codegen/templates/**`, `cmd/fabrica/**`, `pkg/**`, `docs/**`, `examples/**` | anything in a downstream user project |
| **B. Building a service WITH Fabrica** (a generated project, e.g. `user-service/`) | `apis/<group>/<version>/*_types.go`, `internal/storage/storage.go`, `cmd/server/main.go` above the `// Generated` marker, your own non-generated files | any file ending in `_generated.go` |

Determine which job you are on **before** you start. Signal: does the repo root contain
`.fabrica.yaml`? If yes, you are in job B (a generated project). If the repo root contains
`pkg/codegen/`, you are in job A.

---

## 2. Golden rules (non-negotiable)

1. **Never hand-edit a `*_generated.go` file.** It is overwritten on the next
   `fabrica generate` and your change silently disappears. Fix the *template* (job A) or
   the *resource definition* (job B) instead.
2. **`fabrica generate` is idempotent.** Running it twice with no input change produces no
   diff. If it does produce a diff, that is a bug — report it.
3. **Always run the freshly built CLI by absolute path** when validating template changes:
   `/Users/you/fabrica/bin/fabrica`, not whatever `fabrica` resolves to on `$PATH`. Running a
   stale installed binary is the single most common cause of "my template change did nothing."
4. **Use `go run ./cmd/server/`** — with the trailing slash. There are multiple files in that
   package and omitting it fails.
5. **Every new source file needs an SPDX header** (see §7). CI enforces this via the REUSE check.
6. **Do not invent APIs.** Before calling something in `pkg/events`, `pkg/validation`,
   `pkg/versioning`, or `pkg/conditional`, grep for the symbol. Hallucinated helpers
   (`NewValidator`, `WithVersion`, `GetVersion`) are a recurring failure mode.
7. **Secrets never live in resource `Spec` fields as plaintext.** See §8.

---

## 3. The CLI, and exactly when to re-run `fabrica generate`

### Command surface

```
fabrica init <name> [--api-group --module --storage file|ent --db-driver sqlite|postgres|mysql
                     --events --events-bus memory --auth --metrics
                     --validation-mode strict|warn|disabled
                     --version-strategy header|url|both --interactive]
fabrica add resource <Name> [--version v1alpha1 --with-status --with-validation
                             --with-versioning --package <pkg> --force]
fabrica add version <vNext> [--from <vPrev> --force]
fabrica generate [--handlers --storage --client --openapi]
                 [--debug --force --fabrica-source <path>]
fabrica migrate yaml-tags [--dir . --dry-run]
fabrica ent migrate [--dry-run --auto-approve]
fabrica ent describe
fabrica mcp
fabrica version
```

`fabrica ent generate` is **deprecated** — it runs automatically as part of `fabrica generate`.

### Regeneration decision table

`fabrica generate` reads: `.fabrica.yaml`, `apis.yaml`, `apis/<group>/<version>/*_types.go`,
and `go.mod`. **Any change to those inputs requires a regenerate.**

| You changed | Re-run `fabrica generate`? |
|---|---|
| A `Spec` or `Status` struct field in `*_types.go` | **Yes** |
| Added/removed a resource (`fabrica add resource`) | **Yes** |
| Added an API version (`fabrica add version`) | **Yes** |
| Any `+fabrica:` annotation comment | **Yes** |
| A json/yaml struct tag or validation tag | **Yes** |
| `.fabrica.yaml` (enabled events, auth, metrics, switched storage backend) | **Yes** |
| `apis.yaml` (storage version, group name) | **Yes** |
| Bumped the Fabrica dependency in `go.mod` | **Yes** |
| A template under `pkg/codegen/templates/**` (job A) | **Yes**, after `make build`, in a scratch project |
| Business logic in a non-generated file | No |
| `cmd/server/main.go` above the `// Generated` marker | No |
| `internal/storage/storage.go` backend wiring | No |
| README / docs | No |

**Rule of thumb for agents:** if you touched anything under `apis/`, or either YAML config
file, regenerate before you build, test, or declare the task done.

### What generate writes (all overwritten, all off-limits)

- `cmd/server/routes_generated.go`, `*_handlers_generated.go`, `models_generated.go`,
  `openapi_generated.go`, `*_middleware_generated.go`, `event_bus_generated.go`
- `cmd/server/export.go`, `cmd/server/import.go` (offline backup/restore **server
  subcommands**, e.g. `./myapi export` — these are *not* Fabrica CLI commands)
- `internal/storage/storage_generated.go`; for Ent: `ent_adapter.go`,
  `ent_queries_generated.go`, `ent_transactions_generated.go`, `ent/schema/*`
- `pkg/client/client_generated.go`, `pkg/resources/register_generated.go`,
  `pkg/apiversion/registry_generated.go`, `pkg/reconcilers/*_generated.go`

Generated files carry a `// Code generated by Fabrica ... DO NOT EDIT.` banner. Treat that
banner as a hard stop.

---

## 4. Standard workflows

### Job A: change a template and prove it works

```bash
make build                                   # -> bin/fabrica
cd $(mktemp -d) && /abs/path/fabrica/bin/fabrica init scratch --api-group example.com
cd scratch
go mod edit -replace github.com/openchami/fabrica=/abs/path/fabrica && go mod tidy
/abs/path/fabrica/bin/fabrica add resource Widget
/abs/path/fabrica/bin/fabrica generate
go build ./... && go run ./cmd/server/
```

The `-replace` directive is mandatory — without it the scratch project pulls a published
Fabrica and your local change is not exercised. It also suppresses the version-compatibility
warning, which is why `--force` is rarely needed.

Then, back in this repo: `make lint && make test`.

### Job B: add a resource to a service

```bash
fabrica add resource Node --version v1alpha1 --with-status
$EDITOR apis/<group>/v1alpha1/node_types.go   # fill in Spec + Status
fabrica generate
go build ./... && go test ./...
```

---

## 5. Behavioral contracts you must not break

- **List endpoints return a flat JSON array**, not a Kubernetes-style `{"items":[...]}`
  object. Do not "fix" this.
- **Status is a subresource.** Spec updates and status updates are distinct operations.
  See `docs/status-subresource.md`.
- **Events:** construct the bus with `events.NewInMemoryEventBus(buffer, workers)`.
  `Subscribe(eventType, handler)` returns `(SubscriptionID, error)`. `Close()` takes no
  context. Publish through `pkg/events` helpers (`PublishResourceCreated`, …) — never build
  CloudEvents by hand in generated server code.
- **Conditions:** use `resource.Condition`. Condition transitions emit events when events
  are enabled.
- **Conditional requests / PATCH** go through `pkg/conditional`; ETag preconditions are wired
  in middleware, not in handlers.
- **Endpoints default to `/resource-plural`** (e.g. `/sensors`).

---

## 6. Storage

- File backend: `pkg/storage/file_backend.go`.
- Ent backend (v0.4.0+): a generic `Resource` table with `Label` and `Annotation` edges.
  Schemas land in `internal/storage/ent/schema/`, conversion in `internal/storage/ent_adapter.go`,
  per-resource query builders (`QueryServers()`, `ListServersByLabels()`, `GetServerByUID()`)
  in `ent_queries_generated.go`, and `WithTx()` in `ent_transactions_generated.go`.
  Generator entry point: `GenerateEntHelpers()` in `pkg/codegen/generator.go`.
  Templates: `pkg/codegen/templates/storage/ent*.go.tmpl`.
- SQLite examples need `?_fk=1` on the DSN and a pre-existing `data/` directory.
- Export/import work **offline against storage directly** — no running API server. Export uses
  `storage.Query{Resource}(ctx).All(ctx)`; import uses `storage.GetBackend().Save()`.

---

## 7. Coding standards

**License header.** First lines of every new Go file:

```go
// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
```

Markdown and YAML use the equivalent comment syntax (see the top of this file). `REUSE.toml`
covers bulk doc directories; new top-level files generally need their own header. The
`reuse` CI workflow will fail the PR otherwise.

**Lint.** `.golangci.yaml` (v2) enables `errcheck`, `govet`, `ineffassign`, `staticcheck`,
`unused`, `revive`, `misspell`, with `max-issues: 0`. `examples/` and `test/integration/` are
excluded. Run `make lint` (or `make lint-fix`) before finishing.

**Make targets.** `build`, `test`, `test-integration`, `test-all`, `test-coverage`, `lint`,
`lint-fix`, `tidy`, `clean`, `install`, `docker-build`, `act-all` (local CI via `act`).

**Errors.** Wrap with context: `fmt.Errorf("failed to open secret store %s: %w", path, err)`.
Do not discard errors to satisfy `errcheck` — handle or log them, including deferred
`Close()` calls.

**Tests.** Unit tests live beside the code and exclude `examples/`. Integration tests live in
`test/integration/` and follow four phases: generation → server runtime → client binary →
features. Use the `TestProject` helper for project lifecycle. See
`test/integration/TESTING_STRATEGY.md`.

**Templates.** Organized by feature under `pkg/codegen/templates/{server,client,storage,middleware,reconciliation,authorization}/`.
When changing one, cite the exact path in your summary and regenerate a minimal sample to prove it.

**CI.** `tests.yml`, `lint.yaml`, `reuse.yaml`, `govulncheck.yaml`, `scorecard.yaml`,
`release.yaml` (on `v*.*.*` tags, via GoReleaser).

---

## 8. Secrets — use the Magellan secret store

**Directive: Fabrica projects do not implement their own secret storage or encryption.**
The OpenCHAMI ecosystem standard is the Magellan secret store. If a task requires storing
credentials, tokens, BMC passwords, API keys, or any other secret material, use it. Do not
write bespoke AES code, do not add a new crypto dependency, and do not persist secrets in a
resource `Spec`.

### Import path and API

```go
import "github.com/OpenCHAMI/magellan/pkg/secrets"
```

Source of truth: `github.com/OpenCHAMI/magellan`, package `pkg/secrets`.

```go
const DEFAULT_KEY = "default"

type SecretStore interface {
    GetSecretByID(secretID string) (string, error)
    StoreSecretByID(secretID, secret string) error
    ListSecrets() (map[string]string, error)
    RemoveSecretByID(secretID string) error
}
```

Secret values are **opaque strings**. The convention for credential pairs is a JSON document:
`{"username":"...","password":"..."}`.

### Implementations

| Type | Constructor | Use for |
|---|---|---|
| `*LocalSecretStore` | `NewLocalSecretStore(masterKeyHex, filename string, create bool)` or `OpenStore(filename string)` | Real persistence. Encrypted JSON file on disk. |
| `*StaticStore` | `NewStaticStore(username, password string)` | A single credential pair supplied via flags/env. Writes are no-ops. Tests and CLI overrides. |

Helpers:

- `GenerateMasterKey() (string, error)` — 32 random bytes, hex-encoded (AES-256).
- `OpenStore(filename)` — reads the master key from the **`MASTER_KEY` environment
  variable**, creates the file if absent, returns a `SecretStore`. Prefer this in service code.
- `SaveSecrets(jsonFile string, store map[string]string) error` — flushes an in-memory map.

### Crypto properties (for reviewers — do not reimplement)

- Master key: 32 bytes, hex string, supplied via `MASTER_KEY`.
- Per-secret key derivation: HKDF-SHA256 with the `secretID` as salt — every secret gets a
  distinct AES-256 key.
- Encryption: AES-GCM, random nonce prepended, hex-encoded on disk.
- `LocalSecretStore` is guarded by a `sync.RWMutex` and is safe for concurrent use.
- Note: `ListSecrets()` on `LocalSecretStore` returns the **encrypted** values. Call
  `GetSecretByID` to decrypt.

### Canonical usage pattern

Follow Magellan's own precedence: explicit flags win, then a per-ID secret, then `DEFAULT_KEY`.

```go
var store secrets.SecretStore
var err error

if username != "" && password != "" {
    store = secrets.NewStaticStore(username, password)
} else {
    if store, err = secrets.OpenStore(secretsFile); err != nil {
        return fmt.Errorf("failed to open secret store %s: %w", secretsFile, err)
    }
}

raw, err := store.GetSecretByID(id)
if err != nil {
    // fall back to the shared default credential
    raw, err = store.GetSecretByID(secrets.DEFAULT_KEY)
    if err != nil {
        return fmt.Errorf("no credentials for %s and no default: %w", id, err)
    }
}

var creds struct {
    Username string `json:"username"`
    Password string `json:"password"`
}
if err := json.Unmarshal([]byte(raw), &creds); err != nil {
    return fmt.Errorf("malformed credential payload for %s: %w", id, err)
}
```

Bootstrapping a store:

```bash
export MASTER_KEY=$(magellan secrets generatekey)   # or secrets.GenerateMasterKey()
magellan secrets store default "$user:$pass" -f secrets.json
```

### Rules for secret handling in Fabrica projects

1. Store a **reference** (a secret ID) in the resource `Spec`, never the secret itself.
   Resolve it through the `SecretStore` at the point of use.
2. Mark any field that could carry sensitive material with `+fabrica:field:sensitive` so it is
   excluded from logs and debug output. See `pkg/annotations/README.md` and
   `examples/12-storage-annotations/`.
3. For at-rest encryption of a stored field, use the annotation rather than custom code:
   `+fabrica:field:storage=encrypted:aes256:key=vault` (key sources: `env`, `vault`, `kms` —
   validated in `pkg/annotations/validator.go`).
4. Never log a secret, an error string that embeds a secret, or a full credential payload.
   Log the secret **ID** only.
5. `MASTER_KEY` and `secrets.json` are operator-supplied. Never commit either, never generate
   a default value into a template, and add both to `.gitignore` in generated projects.
6. If you are asked to add secret handling and the project does not yet depend on Magellan,
   add `github.com/OpenCHAMI/magellan` to `go.mod` — do not vendor or copy the code.

---

## 9. Common pitfalls

- Running a stale `fabrica` from `$PATH` instead of `bin/fabrica`.
- Forgetting `go mod edit -replace github.com/openchami/fabrica=<local path>` in a scratch project.
- Omitting the trailing slash in `go run ./cmd/server/`.
- Expecting `{"items": [...]}` in list responses — they are bare arrays.
- Editing a `*_generated.go` file instead of the template that produced it.
- Assuming `export`/`import` are Fabrica CLI commands. They are subcommands of the **generated
  server binary**.
- Using `fabrica ent generate` — deprecated, folded into `fabrica generate`.
- Forgetting to regenerate after editing `apis/**` or either YAML config.

---

## 10. Definition of done

Before you report a task complete:

- [ ] `fabrica generate` re-run if any input in §3's table changed, and it produced a clean
      second run (idempotent).
- [ ] `go build ./...` succeeds.
- [ ] `make lint` clean.
- [ ] `make test` passes (add `make test-integration` for template or CLI changes).
- [ ] SPDX headers on every new file.
- [ ] No `*_generated.go` file appears in your diff as a hand edit.
- [ ] No secret, master key, or credential in the diff.
- [ ] Docs and the matching `examples/**/README.md` updated if behavior changed.
- [ ] Summary names the exact files touched.

---

## 11. Key references

| Topic | Path |
|---|---|
| CLI reference | `docs/reference/cli.md` |
| Architecture | `docs/reference/architecture.md` |
| Codegen engine | `pkg/codegen/generator.go`, `docs/reference/codegen.md` |
| Templates | `pkg/codegen/templates/**` |
| Resource model | `docs/guides/resource-model.md` |
| Storage | `docs/guides/storage.md`, `docs/guides/storage-ent.md` |
| Events | `docs/guides/events.md` |
| Reconciliation | `docs/guides/reconciliation.md` |
| Versioning | `docs/guides/versioning.md`, `docs/guides/spec-versioning.md`, `docs/apis-yaml.md` |
| Conditional/PATCH | `docs/guides/conditional-and-patch.md` |
| Annotations (incl. `sensitive`, `encrypted`) | `pkg/annotations/README.md`, `examples/12-storage-annotations/` |
| Testing strategy | `test/integration/TESTING_STRATEGY.md` |
| MCP mode | `docs/reference/mcp.md` |
| Magellan secret store | `github.com/OpenCHAMI/magellan` → `pkg/secrets/` |

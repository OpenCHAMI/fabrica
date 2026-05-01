<!--
Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica MCP Reliability Roadmap

Status snapshot (2026-05-01):

- Completed: core MCP server migration to go-sdk, workspace-scoped tooling, transport compatibility layer, protocol negotiation regression tests, and baseline unit coverage for framing and tool error contracts.
- Partially completed: capability conformance checks and release discipline documentation.
- Not started: real-client contract matrix, CI-enforced MCP contract gate, and named reliability ownership.

## Goals

- Make MCP startup/handshake deterministic across supported clients.
- Treat MCP as a versioned interface with explicit compatibility guarantees.
- Prevent regressions through CI contract tests and release gates.

## Reliability SLOs

- MCP startup success rate: >= 99.9% in CI compatibility matrix runs. (Not yet measurable; matrix job not implemented.)
- Handshake latency p95: <= 500ms in local stdio integration tests. (Not yet measurable; no latency benchmark test in CI.)
- Zero known regressions for previously fixed handshake failures. (Addressed by framed/raw initialize regression tests and end-to-end serve initialize flow coverage.)

## Phase 1: Transport And Handshake Correctness

- [x] Maintain compatibility for both `Content-Length` framing and raw JSON-envelope input.
  Current state: implemented via auto-detect transport reader/writer around the SDK server.
- [x] Implement protocol-version negotiation on `initialize`.
  Current state: Fabrica follows the bundled MCP SDK policy, documents the negotiation behavior, and tests both supported-version preservation and unsupported-version fallback.
- [x] Add structured debug logs behind `FABRICA_MCP_DEBUG=1`.
  Current state: debug logging is wired in server startup and framed output paths.
- Regression tests status:
  - [x] framed initialize request (covered by unit tests for Content-Length input)
  - [x] raw JSON initialize request (covered by dedicated raw JSON initialize input test)
  - [x] end-to-end `serve()` initialize flow (`serveWithIO` test path)

## Phase 2: Compatibility Matrix And Contract Tests

- [ ] Add MCP contract tests that run `fabrica mcp` against real client libraries.
- [ ] Track matrix by:
  - Fabrica version
  - MCP client implementation/version
  - OS/runtime
- [ ] Publish known-good combinations in docs.

## Phase 3: Capability Coverage

- [~] Explicitly declare and test tool, resource, and prompt capabilities as needed.
  Current state: tools/resources capabilities are declared; prompt capabilities are not declared; tests focus on tools and transport helpers.
- [~] Validate that capability declarations match implemented methods.
  Current state: tool list/tests exist, but no dedicated capabilities conformance test suite.
- [~] Add conformance checks for error payload shape and method-not-found behavior.
  Current state: structured tool error payload shape is tested; method-not-found behavior at protocol layer is not explicitly tested.

## Phase 4: Release Discipline

- [ ] Add mandatory CI job: `mcp-contract`.
- [ ] Require changelog entries for MCP behavior changes.
  Current state: changelog exists, but there is no enforcement policy or CI check for MCP-specific entries.
- [ ] Block release if contract tests fail on any supported matrix target.

## Ownership

- [ ] Assign one maintainer as MCP reliability owner.
- [ ] Require code-owner review for files under `internal/mcp/*` and MCP docs.
  Current state: no CODEOWNERS file is present in the repository.

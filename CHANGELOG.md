<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.2.9] - 2025-10-27

### Fixed
- CloudEvents event-bus template aligned with current events package API
  - Switched to `events.NewInMemoryEventBus(buffer, workers)` for the in-memory bus
  - `Publish` now receives `events.Event` (no direct CloudEvents struct leakage)
  - `Subscribe` signature corrected to return `(SubscriptionID, error)` and no context argument
  - `Close()` called without context parameter
- Middleware templates updated to compile against actual APIs
  - Validation middleware uses the functions from `pkg/validation` correctly (no `NewValidator` stub)
  - Versioning middleware replaced non-existent `WithVersion`/`GetVersion` calls with the proper helpers
  - Conditional middleware cleans up unused imports when not required

### Changed
- Generated main template initializes the event system and event bus on startup when events are enabled
- CloudEvents example README now matches the generated API
  - Sensor spec includes `sensorType`, `location`, and `threshold`
  - Status examples use `resource.Condition` for condition changes
  - Clarified that list endpoints return arrays and showed accurate curl examples
- FRU example and other READMEs
  - Fixed SQLite foreign key configuration (`?_fk=1`) and ensured data directory setup
  - Normalized server run commands to `go run ./cmd/server/` (trailing slash)
  - Documented adding a `go mod edit -replace` directive before `go mod tidy` when testing with a local fabrica checkout

### Documentation
- Updated example READMEs (01-basic-crud, 03-fru-service, 05-cloud-events) for accuracy and troubleshooting
- examples/README.md refreshed to reflect the verified workflows

### Notes
- No breaking changes to the fabrica CLI. Projects generated with prior versions that hit event bus API mismatches can be fixed by regenerating code with v0.2.9.

### Added
- **Status Subresource Pattern** - Kubernetes-style status management ✨
  - Separate endpoints for spec (`PUT /resources/{uid}`) and status (`PUT /resources/{uid}/status`) updates
  - Prevents conflicts between user updates and controller/reconciler updates
  - Enhanced `BaseReconciler.UpdateStatus()` to load fresh resource and preserve spec changes
  - Generated client library includes `UpdateResourceStatus()` and `PatchResourceStatus()` methods
  - Support for fine-grained authorization via optional `StatusPolicy` interface
  - Status updates publish events with `updateType: "status"` metadata for differentiation
  - Comprehensive documentation in `docs/status-subresource.md`
  - Example implementation in `examples/06-status-subresource/`
  - Integration tests for spec/status separation in `test/integration/status_subresource_test.go`
  - unified previous Conditions API and Cloud-Events tooling so every status update triggers a cloud-event publish

### Changed
- **Automatic Ent Generation** - Simplified Ent storage workflow
  - `fabrica generate` now automatically runs Ent client code generation when Ent storage is detected
  - Provides consistent single-command workflow across all storage backends
- **Template Organization** - Improved codebase maintainability
  - Reorganized templates into feature-based directory structure
  - Server templates: `server/` (handlers, routes, models, openapi)
  - Client templates: `client/` (client, models, cmd)
  - Storage templates: `storage/` (file, ent, adapter, generate)
  - Middleware templates: `middleware/` (validation, conditional, versioning, event-bus)
  - Reconciliation templates: `reconciliation/` (reconciler, stub, registration, event-handlers)
  - Authorization templates: `authorization/` (policies, model.conf, policy.csv)
  - Standardized all template names to use hyphens consistently
  - Removed unused `policy_handlers.go.tmpl` template
- Updated `Update{Resource}()` handler documentation to clarify it updates spec only
- Enhanced reconciler patterns to use status-only updates by default

### Documentation
- Added comprehensive [Status Subresource Guide](docs/status-subresource.md) with:
  - Architecture overview and problem/solution explanation
  - API usage examples (curl and client library)
  - Reconciler patterns and best practices
  - Authorization examples with Casbin
  - Event semantics and subscription patterns
  - Troubleshooting guide
- Added [Example 6: Status Subresource](examples/06-status-subresource/README.md)
- Updated main documentation index to include status subresource guide
- Added implementation guide in `.claude/status-subresource-implementation-guide.md`
- Updated [Ent Storage Guide](docs/storage-ent.md) to reflect automatic Ent generation
- Updated [Example 3: FRU Service](examples/03-fru-service/README.md) to remove manual Ent generation step
- Added [Command Structure Analysis](.claude/command-structure-analysis.md) documenting the consolidation rationale
- Added [Template Usage Analysis](.claude/template-usage-analysis.md) documenting template organization and cleanup

### Deprecated
- `fabrica ent generate` command is deprecated in favor of automatic generation during `fabrica generate`
  - Still functional for backward compatibility
  - Will be removed in v0.4.0
  - Displays deprecation warning when used

## [v0.2.8] - 2025-10-20

### Fixed
- Fixed reconciler code generation templates
- Fixed integration test for rack reconciliation
- Fixed integration test expectations to properly validate both generated and stub reconciler files
- Fixed Ent storage integration test to use `fabrica ent generate` command instead of direct `go generate`

### Changed
- Removed automatic `go mod tidy` execution from `fabrica generate` command to avoid circular dependency issues
- Modified workflow to make `go mod tidy` a user responsibility after code generation
- Updated `fabrica ent generate` helper in integration tests to accept binary path parameter

### Added
- Added stub files for Ent schema sub-packages (`annotation`, `label`, `resource`) during `fabrica init --storage-type ent`
- Added `GenerateEnt()` helper method in integration test utilities
- Added instructions in success messages to run `go mod tidy` after `fabrica generate`

### Documentation
- Updated README.md to include `go mod tidy` step in quickstart workflow
- Updated docs/quickstart.md with dependency resolution step
- Updated docs/getting-started.md with proper workflow steps
- Updated docs/storage-ent.md to clarify `fabrica ent generate` usage
- Updated all example READMEs to include `go mod tidy` in workflows (examples/01-basic-crud, examples/03-fru-service, examples/04-rack-reconciliation)
- Updated examples/README.md with complete workflow including dependency management

## [v0.2.4] - 2025-10-06

### Added
- Makefile for building fabrica with version information from git tags
- Support for initializing fabrica projects in existing directories
- Casbin RBAC authorization infrastructure in code generation
  - `--auth` flag for `fabrica init` to enable authorization
  - Auto-generation of Casbin policy files (model.conf, policy.csv)
  - Authorization middleware hooks in generated handlers
  - Policy registry and auth context helpers in generated code

### Changed
- Code generation templates refactored for improved storage handling
- Storage templates now use proper fabrica storage backend interface
- Handler templates include authorization checks when auth is enabled
- Improved go.mod generation with proper semantic versions instead of "latest"

### Removed
- Outdated getting started documentation
- Legacy example projects that didn't reflect current architecture

## [v0.2.3] - 2025-10-05

### Added
- Go Report Card badge to README
- OpenSSF Scorecard badge to README
- Authorization policy integration and management handlers

## [v0.2.2] - 2025-10-04

### Changed
- Updated version references to v0.2.2
- Updated Docker image references

## [v0.2.1] - 2025-10-04

### Changed
- Updated version to v0.2.1
- Updated Docker image references

## [v0.2.0] - 2025-10-04

### Changed
- Updated documentation for v0.2.0 release
- Updated configuration for v0.2.0 release
- Cleaned up codebase for v0.2.0 release

## [v0.1.0] - 2025-10-04

### Added
- Initial release of Fabrica framework
- Core resource model with Kubernetes-style API versioning
- Resource metadata system (UID, labels, annotations)
- Multi-version schema support with automatic conversion
- Storage backend abstraction
  - File-based storage backend
  - Ent ORM storage backend support
- Validation framework
  - Struct tag validation
  - Custom business logic validation
  - Context-aware validation
- Events and reconciliation framework
- PATCH operation support with middleware
- Casbin RBAC policy management
- Code generation capabilities
  - Handler generation
  - Storage adapter generation
  - Route registration
  - OpenAPI specification generation
- Comprehensive documentation
  - Resource model documentation
  - Storage system documentation
  - Versioning documentation
  - Framework comparison guide
- CI/CD configuration
  - golangci-lint configuration
  - GoReleaser configuration
  - GitHub Actions workflows
- Project badges
  - Build status
  - Go Report Card
  - License information

### Documentation
- Comprehensive framework comparison with other Go frameworks
- Resource model and versioning guide
- Storage system architecture documentation
- Getting started guide

[Unreleased]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.9...HEAD
[v0.2.9]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.8...v0.2.9
[v0.2.8]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.4...v0.2.8
[v0.2.4]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.3...v0.2.4
[v0.2.3]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.2...v0.2.3
[v0.2.2]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.1...v0.2.2
[v0.2.1]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/alexlovelltroy/fabrica/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/alexlovelltroy/fabrica/releases/tag/v0.1.0

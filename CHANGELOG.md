<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[Unreleased]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.8...HEAD
[v0.2.8]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.4...v0.2.8
[v0.2.4]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.3...v0.2.4
[v0.2.3]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.2...v0.2.3
[v0.2.2]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.1...v0.2.2
[v0.2.1]: https://github.com/alexlovelltroy/fabrica/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/alexlovelltroy/fabrica/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/alexlovelltroy/fabrica/releases/tag/v0.1.0

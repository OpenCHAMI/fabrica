<!--
SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica Integration Tests

This directory contains Go-based integration tests for the Fabrica API generation framework using testify.

## Overview

These tests verify that Fabrica can successfully:
- Initialize projects with different storage backends
- Add resources to projects
- Generate complete API code
- Build working server and client binaries
- Support multiple resources in a single project

## Test Structure

### Main Test Suite (`clean_test.go`)
- `TestBasicFileStorageGeneration` - File-based storage API generation
- `TestCRUDOperations` - Basic API project generation and building
- `TestEntStorageGeneration` - Database-backed API generation with Ent
- `TestMultipleResources` - Multi-resource API testing
- `TestPatchFormats` - PATCH functionality generation

### Test Helpers (`helpers.go`)
- `TestProject` struct for managing fabrica project lifecycle
- Utilities for project initialization, resource addition, code generation
- Build verification and file existence assertions
- JSON response parsing helpers (ready for future server testing)

## Running Tests

### Prerequisites
- Go 1.23 or later
- Fabrica binary built (`make build` from project root)

### Local Testing
```bash
cd test/integration

# Run all integration tests
go test -v -timeout 10m

# Run specific test
go test -v -timeout 5m -run TestFabricaTestSuite/TestBasicFileStorageGeneration

# Run with more verbose output
go test -v -timeout 10m -args -test.v
```

### CI/CD Integration
Tests run automatically on:
- Pull requests to main branch
- Pushes to main branch

See `.github/workflows/regression-tests.yml` for CI configuration.

## Test Coverage

### ✅ Working Tests
- **Basic File Storage** - Verifies file-based API generation and building
- **CRUD Operations** - Tests complete project generation workflow
- **Ent Storage** - Validates database-backed API generation
- **Multiple Resources** - Ensures multi-resource projects work correctly
- **PATCH Formats** - Confirms PATCH functionality is generated

### 🚧 Future Improvements
- **Server Integration** - Add actual HTTP API testing with running servers
- **README Example** - Fix Ent circular dependency issues in README example
- **Performance Testing** - Add benchmarks for code generation speed
- **Contract Testing** - Verify API contracts match OpenAPI specs

## Architecture Benefits

### Advantages over Bash Scripts
- **Type Safety** - Compile-time error checking and IDE support
- **Rich Assertions** - testify provides expressive test assertions
- **Structured Testing** - Proper setup/teardown with test suites
- **Better Debugging** - Breakpoints, stack traces, and IDE integration
- **Parallel Execution** - Tests can run in parallel for speed
- **JSON Handling** - Native struct marshaling vs string parsing

### Test Organization
- **Reusable Helpers** - `TestProject` encapsulates common patterns
- **Clean Setup/Teardown** - Automatic temp directory management
- **Modular Design** - Easy to add new test scenarios
- **CI/CD Ready** - Native GitHub Actions integration

## Adding New Tests

1. Add new test method to `FabricaTestSuite`
2. Follow pattern: Initialize → Add Resources → Generate → Build → Assert
3. Use `TestProject` helpers for consistency
4. Add file existence assertions for generated code
5. Consider adding to CI workflow if needed

Example:
```go
func (s *FabricaTestSuite) TestNewFeature() {
    project := s.createProject("feature-test", "github.com/test/feature", "file")

    err := project.Initialize(s.fabricaBinary)
    s.Require().NoError(err)

    // Add test-specific logic...
}
```

// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

# Progressive Integration Testing for Fabrica

This directory contains a multi-layered integration test strategy that validates Fabrica across the full generation and runtime lifecycle.

## Testing Phases

### Phase 1: Code Generation Tests (Existing)
**Files:** `generator_test.go`, `clean_test.go`, `regeneration_test.go`, `export_import_test.go`, etc.

**What they test:** Verifies that Fabrica's CLI commands work correctly and generate syntactically valid Go code that compiles.

**Scope:**
- Project initialization with different storage backends
- Resource addition and code regeneration
- Multi-resource projects
- File and Ent storage code generation
- Client and server code compilation
- Export/import command generation

**Execution:** `go test -run TestBasic`

### Phase 2: Server Runtime Tests (New)
**File:** `runtime_test.go`

**What they test:** Verifies that generated API servers actually start and respond correctly, with client library calls working against running servers.

**Key tests:**
- `TestServerStartupAndHealth` - Server starts and health endpoint works
- `TestCRUDViaHTTP` - Full CRUD cycle via HTTP: Create, Read, List, Update, Delete, Patch
- `TestMultiResourceProject` - Multiple resources work independently
- `TestPatchOperations` - PATCH functionality
- `TestFileStorage` - File storage backend creates/maintains files
- `TestErrorHandling` - Appropriate error responses for invalid operations
- `TestOpenAPIGeneration` - OpenAPI spec is generated
- `TestValidationWithInlinedSpecFields` - Validation tags on spec fields work correctly in versioned APIs (tests that validation runs on flattened request envelopes before resource construction)

**Coverage:**
- HTTP request/response handling
- Request validation and error responses
- Resource lifecycle management
- Storage backend integration
- Concurrent resource types

**Execution:** `go test -run TestRuntime`

### Phase 3: Client Binary Tests (New)
**File:** `client_binary_test.go`

**What they test:** Verifies that generated CLI client binaries compile and basic commands execute. This is a smoke test—functional validation relies on library tests.

**Key tests:**
- `TestClientBinaryCompilation` - Binary compiles and is executable
- `TestClientHelpCommand` - `--help` flag works
- `TestClientResourceCommands` - Resource-specific subcommands exist
- `TestClientListCommand` - List command executes against server
- `TestClientMultipleResources` - Client handles multiple resource types
- `TestClientBinaryInProject` - Client source structure is correct

**Coverage:**
- CLI code generation correctness
- Command structure (subcommands and flags)
- Binary execution

**Note:** Functional testing (argument parsing, output formatting) relies on Phase 2 library tests. Phase 3 is purely a smoke test.

**Execution:** `go test -run TestClientBinary`

### Phase 4: Feature-Specific Runtime Tests (Future)
Planned but not yet implemented. Would test:
- Middleware (ETag, validation, versioning)
- Event publishing and reconciliation
- Multi-version API workflows
- Concurrent operations
- Database-backed storage (Ent with SQLite)

## Test Infrastructure

### TestProject Helper (`helpers.go`)
Encapsulates project lifecycle:
- **Initialize()** - Create project with fabrica, set up git, inject `go mod replace` directives
- **AddResource()** - Dynamically add resources
- **Generate()** - Run code generation
- **StartServerRuntime()** - Compile and start generated server, wait for health check
- **StopServer()** - Kill running server
- **HTTPCall()** - Generic HTTP request helper with headers and body support
- **BuildClientBinary()** - Compile generated client CLI
- **RunClientBinary()** - Execute client with arguments

### Shared Bash Library (`test-lib.sh`)
Reusable functions for example scripts:
- **wait_for_server()** - Poll health endpoint with timeout
- **http_get/post/put/patch/delete()** - HTTP request helpers
- **assert_status_code()** - Verify HTTP response codes
- **test_crud_operations()** - Standardized CRUD test flow
- **Color output functions** - Consistent formatting
- **Cleanup utilities** - Server process management

## Port Management

Tests use `findAvailablePort()` to dynamically allocate ports, enabling:
- Parallel test execution without conflicts
- No hardcoded port assumptions
- Automatic cleanup on test exit

## Storage Backend Testing

- **Phase 2 & 3:** Default to file storage for speed and isolation
- **Future Phase 4:** Will include `:memory:` SQLite variants for database testing
- All tests use isolated temporary directories

## Running Tests

### All integration tests
```bash
go test -v -timeout 10m ./test/integration
```

### Specific phase
```bash
go test -v -run Phase2 ./test/integration  # Runtime tests only
go test -v -run ClientBinary ./test/integration  # Client binary tests only
```

### Single test
```bash
go test -v -run TestServerStartupAndHealth ./test/integration
```

## Prerequisites

1. **Fabrica binary built:** `make build` from project root
2. **Go 1.23+** installed
3. **curl** for HTTP testing (Phase 2, 3)
4. **jq** optional (test-lib.sh uses it for JSON extraction, falls back to grep)

## Integration with Examples

Example test scripts can source `test-lib.sh`:
```bash
source ../../test/integration/test-lib.sh

require_commands curl jq
wait_for_server http://localhost:8080
test_crud_operations "Device" "/devices" '{"spec":{"description":"test"}}'
```

This consolidates common patterns and reduces duplication across examples.

## Coverage Strategy

| Concern | Phase 1 | Phase 2 | Phase 3 | Phase 4 |
|---------|---------|---------|---------|---------|
| **Code Generation** | ✓ | - | - | - |
| **Server Compilation** | ✓ | ✓ | - | - |
| **Server Startup** | - | ✓ | - | - |
| **HTTP Handling** | - | ✓ | - | - |
| **CRUD Operations** | - | ✓ | - | - |
| **Client Compilation** | - | ✓ | ✓ | - |
| **Client Execution** | - | - | ✓ | - |
| **Middleware** | - | - | - | ✓ |
| **Events & Reconciliation** | - | - | - | ✓ |
| **Multi-Version APIs** | - | - | - | ✓ |
| **Database Backends** | - | - | - | ✓ |

## Future Enhancements

1. **Test categorization:** Label tests by feature (CRUD, events, versioning) for targeted runs
2. **Coverage reports:** Integrate with Go coverage tooling for code generation coverage
3. **Benchmark tests:** Performance validation for generated code
4. **E2E examples:** Automated example walkthroughs from example READMEs
5. **Parallel execution:** Optimize timeout settings for parallel test runs
6. **CI integration:** GitHub Actions workflow consuming these tests
7. **Middleware validation tests:** More comprehensive tests for ETag, conditional requests, and versioning middleware
8. **Update/Patch validation:** Test that validation also works correctly for PUT and PATCH operations with inlined spec fields

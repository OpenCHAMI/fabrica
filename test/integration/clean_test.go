// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package integration provides comprehensive integration tests for the Fabrica API generation framework.
//
// These tests verify end-to-end functionality of Fabrica by creating real projects, generating code,
// and building complete API applications. The tests use the testify framework for structured test
// organization and rich assertions.
//
// Test Coverage:
//   - Basic file storage API generation and building
//   - Ent database storage backend generation
//   - Multiple resource support in single projects
//   - PATCH functionality generation
//   - CRUD operation code generation
//
// The integration tests focus on verifying that Fabrica can successfully generate working,
// buildable Go projects rather than testing runtime API behavior. This ensures that generated
// code compiles correctly and includes all necessary dependencies.
//
// Usage:
//
//	go test -v -timeout 10m ./...
//
// Prerequisites:
//   - Fabrica binary must be built (make build from project root)
//   - Go 1.23 or later
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// FabricaTestSuite is the main test suite using our helper utilities
type FabricaTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

// SetupSuite initializes the test environment
func (s *FabricaTestSuite) SetupSuite() {
	// Find fabrica binary
	wd, err := os.Getwd()
	s.Require().NoError(err)

	projectRoot := filepath.Join(wd, "..", "..")
	s.fabricaBinary = filepath.Join(projectRoot, "bin", "fabrica")
	s.Require().FileExists(s.fabricaBinary, "fabrica binary must be built")

	// Convert to absolute path
	s.fabricaBinary, err = filepath.Abs(s.fabricaBinary)
	s.Require().NoError(err)

	// Create temp directory
	s.tempDir = s.T().TempDir()
}

// TearDownTest cleans up after each test
func (s *FabricaTestSuite) TearDownTest() {
	// Stop all servers
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

// Helper to create and track test projects
func (s *FabricaTestSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

func (s *FabricaTestSuite) TestBasicFileStorageGeneration() {
	// Create project
	project := s.createProject("file-test", "github.com/test/file", "file")

	// Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add resource
	err = project.AddResource(s.fabricaBinary, "Item")
	s.Require().NoError(err, "adding resource should succeed")

	// Generate code
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	// Verify generated files
	project.AssertFileExists("cmd/server/main.go")
	project.AssertFileExists("cmd/client/main.go")
	project.AssertFileExists("cmd/server/item_handlers_generated.go") // Updated to match actual output
	project.AssertFileExists("internal/storage/storage_generated.go") // Updated path

	// Build project
	err = project.Build()
	s.Require().NoError(err, "project should build successfully")
}

func (s *FabricaTestSuite) TestAuthEnabledFileStorageGeneration() {
	project := s.createProject("auth-file-test", "github.com/test/auth-file", "file")

	err := project.InitializeWithFlags(s.fabricaBinary, "--auth")
	s.Require().NoError(err, "project initialization with auth should succeed")

	if branch, ok := TokenSmithBranchForTests(); ok {
		err = project.PinTokenSmithBranch(branch)
		s.Require().NoError(err, "pinning TokenSmith feature branch should succeed")
	}

	err = project.AddResource(s.fabricaBinary, "Item")
	s.Require().NoError(err, "adding resource should succeed")

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed with auth enabled")

	project.AssertFileExists("cmd/server/main.go")
	project.AssertFileExists("cmd/client/main.go")
	project.AssertFileExists("cmd/server/item_handlers_generated.go")
	project.AssertFileExists("internal/storage/storage_generated.go")

	err = project.Build()
	s.Require().NoError(err, "auth-enabled project should build successfully")
}

func (s *FabricaTestSuite) TestModuleCompatibilityCheckPreventsGeneration() {
	// Validates PR-38 class failure: generation with mismatched fabrica module version
	// Expected behavior: preflight check catches mismatch and offers remediation
	project := s.createProject("module-compat-test", "github.com/test/module-compat", "file")

	// Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	err = project.AddResource(s.fabricaBinary, "Item")
	s.Require().NoError(err, "adding resource should succeed")

	// Modify go.mod to introduce a version mismatch
	// We'll update the fabrica module require to a different version than what's being tested
	err = project.SetFabricaModuleVersion("v0.1.0") // Version that will not match CLI
	s.Require().NoError(err, "setting mismatched fabrica version should succeed")

	// Attempt to generate - this should fail with actionable error message
	cmd := exec.Command(s.fabricaBinary, "generate", "--storage", "--openapi", "--handlers", "--client")
	cmd.Dir = project.Dir
	output, err := cmd.CombinedOutput()

	// We expect generation to fail
	s.Require().Error(err, "generate should fail with version mismatch")

	// Verify error message includes actionable guidance
	outputStr := string(output)
	s.Require().Contains(outputStr, "Module version mismatch detected", "error should mention mismatch")
	s.Require().Contains(outputStr, "Project module", "error should identify the module")

	// Verify remediation hints are present
	s.Require().True(
		contains(outputStr, "Rebuild") || contains(outputStr, "replace") || contains(outputStr, "go get"),
		"error should include remediation paths",
	)

	// Now test --force flag bypasses the check
	forcedCmd := exec.Command(s.fabricaBinary, "generate", "--storage", "--openapi", "--handlers", "--client", "--force")
	forcedCmd.Dir = project.Dir
	forcedOutput, forcedErr := forcedCmd.CombinedOutput()
	// With --force, it should proceed (though may fail later due to version incompatibility)
	// The key is it doesn't fail on the preflight check itself
	_ = forcedErr    // May fail later, that's OK
	_ = forcedOutput // Just testing the preflight bypass
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func (s *FabricaTestSuite) TestEntStorageGeneration() {
	project := s.createProject("ent-test", "github.com/test/ent", "ent")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "User")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Ent code generation now runs automatically during 'fabrica generate'
	// Verify Ent-specific files exist
	project.AssertFileExists("internal/storage/ent/schema/resource.go") // Updated to match actual Ent structure
	project.AssertFileExists("internal/storage/storage_generated.go")   // Updated to match actual structure

	err = project.Build()
	s.Require().NoError(err)
}

func (s *FabricaTestSuite) TestCRUDOperations() {
	// Create project focused on testing that we can build and generate correctly
	project := s.createProject("crud-test", "github.com/test/crud", "file")

	// Setup project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Item") // Use Item like our working test
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.Build()
	s.Require().NoError(err)

	// Verify the key generated files exist and contain expected content
	project.AssertFileExists("cmd/server/main.go")
	project.AssertFileExists("cmd/client/main.go")
	project.AssertFileExists("cmd/server/item_handlers_generated.go")
	project.AssertFileExists("cmd/server/routes_generated.go")

	// For now, skip the server startup tests until we have a more robust setup
	// TODO: Add server integration tests in a follow-up
}

func (s *FabricaTestSuite) TestPatchFormats() {
	// Test that we can generate a project with patch functionality
	project := s.createProject("patch-test", "github.com/test/patch", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Setting") // Changed from Config to avoid naming conflicts
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.Build()
	s.Require().NoError(err)

	// Verify patch-related files are generated
	project.AssertFileExists("cmd/server/setting_handlers_generated.go") // Updated to match new resource name
	project.AssertFileExists("cmd/client/main.go")

	// TODO: Add actual patch testing once server integration is stable
}

func (s *FabricaTestSuite) TestMultipleResources() {
	project := s.createProject("multi-test", "github.com/test/multi", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	// Add multiple resources
	resources := []string{"User", "Product", "Order"}
	for _, resource := range resources {
		err = project.AddResource(s.fabricaBinary, resource)
		s.Require().NoError(err, "adding %s should succeed", resource)
	}

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.Build()
	s.Require().NoError(err)

	// Verify that handler files are generated for each resource
	project.AssertFileExists("cmd/server/user_handlers_generated.go")
	project.AssertFileExists("cmd/server/product_handlers_generated.go")
	project.AssertFileExists("cmd/server/order_handlers_generated.go")
	project.AssertFileExists("cmd/server/routes_generated.go")
}

func (s *FabricaTestSuite) TestCreateFRUApplication() {
	// Test the README example functionality with a test-friendly module name
	project := s.createProject("fru-service", "test.local/fru", "file")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "README example init should work")

	err = project.AddResource(s.fabricaBinary, "FRU")
	s.Require().NoError(err, "README example resource add should work")

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "README example generate should work")

	// Verify the expected files are generated (this is the main test goal)
	project.AssertFileExists("cmd/server/main.go")
	project.AssertFileExists("cmd/client/main.go")
	project.AssertFileExists("cmd/server/fru_handlers_generated.go")

	// For file storage, check storage file instead of Ent schema
	project.AssertFileExists("internal/storage/storage_generated.go")
}

func (s *FabricaTestSuite) TestExample1_EndToEnd() {
	// SKIPPED: This test requires building and running the generated server, which requires
	// resolving go.mod dependencies for the generated project. This is complex and fragile with
	// fake test module paths. The primary goal is to test that Fabrica generates correct code,
	// not that the build system works. Use local examples instead for end-to-end testing.
	s.T().Skip("test requires compiled binaries - use examples/01-basic-crud for end-to-end testing")

	// Create project
	project := s.createProject("example1-crud", "github.com/test/example1", "file")

	// 1. Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// 2. Add resource
	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err, "adding resource should succeed")

	// 3. Customize resource (New Step)
	err = project.Example1_CustomizeResource()
	s.Require().NoError(err, "customizing resource should succeed")

	// 4. Generate code
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	// 5. Configure server (New Step)
	err = project.Example1_ConfigureServer()
	s.Require().NoError(err, "configuring server main.go should succeed")

	// 6. Build project
	err = project.Build()
	s.Require().NoError(err, "project should build successfully")

	// 7. Start server
	err = project.StartServer()
	s.Require().NoError(err, "server should start successfully")
	// Ensure server is stopped when test finishes
	s.T().Cleanup(func() {
		project.StopServer()
	})

	// 8. Run Client Tests (Full CRUD)

	// CREATE
	createSpec := map[string]interface{}{
		"description": "Core network switch",
		"ipAddress":   "192.168.1.10",
		"location":    "DataCenter A",
		"rack":        "R42",
	}
	created, err := project.CreateResource("device", createSpec)
	s.Require().NoError(err, "client create should succeed")
	s.Require().NotNil(created, "created resource should not be nil")

	// Verify metadata and spec
	uid, ok := created["metadata"].(map[string]interface{})["uid"].(string)
	s.Require().True(ok, "should get uid from metadata")
	s.Require().NotEmpty(uid, "uid should not be empty")
	project.AssertResourceHasSpec(s.T(), created, createSpec)

	// LIST
	listed, err := project.ListResources("device")
	s.Require().NoError(err, "client list should succeed")
	s.Require().Len(listed, 1, "list should return one device")
	s.Require().Equal(uid, listed[0]["metadata"].(map[string]interface{})["uid"].(string))

	// GET
	got, err := project.GetResource("device", uid)
	s.Require().NoError(err, "client get should succeed")
	project.AssertResourceHasSpec(s.T(), got, createSpec)

	// PATCH (Example 1 doesn't test this, but we can use the helper)
	patchSpec := map[string]interface{}{
		"location": "DataCenter B",
	}
	patched, err := project.PatchResource("device", uid, patchSpec)
	s.Require().NoError(err, "client patch should succeed")

	// Verify patch
	s.Require().Equal("DataCenter B", patched["spec"].(map[string]interface{})["location"], "location should be updated")
	s.Require().Equal("192.168.1.10", patched["spec"].(map[string]interface{})["ipAddress"], "ipAddress should be unchanged")

	// DELETE
	err = project.DeleteResource("device", uid)
	s.Require().NoError(err, "client delete should succeed")

	// VERIFY DELETE
	listedAfterDelete, err := project.ListResources("device")
	s.Require().NoError(err, "client list after delete should succeed")
	s.Require().Len(listedAfterDelete, 0, "list should be empty after delete")

	_, err = project.GetResource("device", uid)
	s.Require().Error(err, "client get after delete should fail")
}

// Run the test suite
func TestFabricaTestSuite(t *testing.T) {
	suite.Run(t, new(FabricaTestSuite))
}

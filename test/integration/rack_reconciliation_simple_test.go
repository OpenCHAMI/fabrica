// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/alexlovelltroy/fabrica/test/integration/testdata"
)

// RackReconciliationSimpleTestSuite tests that rack reconciliation code generation works
type RackReconciliationSimpleTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	project       *TestProject
}

// SetupSuite initializes the test environment
func (s *RackReconciliationSimpleTestSuite) SetupSuite() {
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
func (s *RackReconciliationSimpleTestSuite) TearDownTest() {
	if s.project != nil {
		s.project.StopServer() //nolint:all
	}
}

// TestRackReconciliationCodeGeneration tests code generation for rack reconciliation
func (s *RackReconciliationSimpleTestSuite) TestRackReconciliationCodeGeneration() {
	// Create a test project
	s.project = NewTestProject(&s.Suite, s.tempDir, "rack-reconciliation-test", "github.com/test/rackrecon", "file")

	// Initialize project
	err := s.project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add all required resources
	resources := []string{"RackTemplate", "Rack", "Chassis", "Blade", "BMC", "Node"}
	for _, resource := range resources {
		err = s.project.AddResource(s.fabricaBinary, resource)
		s.Require().NoError(err, "adding %s resource should succeed", resource)
	}

	// Update resource definitions with proper Spec and Status structs
	s.updateResourceDefinitions()

	// Generate code
	err = s.project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	// Verify generated files exist
	s.project.AssertFileExists("cmd/server/main.go")
	s.project.AssertFileExists("cmd/server/racktemplate_handlers_generated.go")
	s.project.AssertFileExists("cmd/server/rack_handlers_generated.go")
	s.project.AssertFileExists("cmd/server/chassis_handlers_generated.go")
	s.project.AssertFileExists("cmd/server/blade_handlers_generated.go")
	s.project.AssertFileExists("cmd/server/bmc_handlers_generated.go")
	s.project.AssertFileExists("cmd/server/node_handlers_generated.go")

	// Add the reconciliation code
	s.addReconciliationCode()

	// Verify reconciler was created
	s.project.AssertFileExists("pkg/reconciler/rack_reconciler.go")

	// Build the project - this proves all the code is valid
	err = s.project.Build()
	s.Require().NoError(err, "project should build successfully with reconciliation code")

	s.T().Log("✓ Rack reconciliation code generation test passed")
	s.T().Log("✓ All resources defined correctly")
	s.T().Log("✓ Reconciler compiles successfully")
	s.T().Log("✓ Project builds with reconciliation support")
}

// updateResourceDefinitions updates the generated resource files with proper specs
func (s *RackReconciliationSimpleTestSuite) updateResourceDefinitions() {
	// Update each resource file with proper Spec and Status structs
	resources := map[string]string{
		"RackTemplate": testdata.RackTemplateResource,
		"Rack":         testdata.RackResource,
		"Chassis":      testdata.ChassisResource,
		"Blade":        testdata.BladeResource,
		"BMC":          testdata.BMCResource,
		"Node":         testdata.NodeResource,
	}

	for resourceName, content := range resources {
		resourcePath := filepath.Join(s.project.Dir, "pkg", "resources", strings.ToLower(resourceName), strings.ToLower(resourceName)+".go")
		err := os.WriteFile(resourcePath, []byte(content), 0644)
		s.Require().NoError(err, "failed to write %s resource definition", resourceName)
		s.T().Logf("Updated resource definition: %s", resourceName)
	}
}

// addReconciliationCode adds the reconciler implementation to the project
func (s *RackReconciliationSimpleTestSuite) addReconciliationCode() {
	// Create reconciler directory
	reconcilerDir := filepath.Join(s.project.Dir, "pkg", "reconciler")
	err := os.MkdirAll(reconcilerDir, 0755)
	s.Require().NoError(err, "failed to create reconciler directory")

	// Write reconciler code with module replacement
	reconcilerCode := strings.ReplaceAll(testdata.RackReconcilerCode, "{{.Module}}", s.project.Module)
	reconcilerPath := filepath.Join(reconcilerDir, "rack_reconciler.go")
	err = os.WriteFile(reconcilerPath, []byte(reconcilerCode), 0644)
	s.Require().NoError(err, "failed to write reconciler code")

	s.T().Log("Added rack reconciler implementation")
}

// Run the test suite
func TestRackReconciliationSimpleTestSuite(t *testing.T) {
	suite.Run(t, new(RackReconciliationSimpleTestSuite))
}

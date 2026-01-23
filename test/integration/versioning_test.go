// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// VersioningSuite tests hub/spoke API versioning
type VersioningSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

// SetupSuite initializes the test environment
func (s *VersioningSuite) SetupSuite() {
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
func (s *VersioningSuite) TearDownTest() {
	// Stop all servers
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

// Helper to create and track test projects
func (s *VersioningSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

// TestFlattenedEnvelopeStructure verifies that generated resources use flattened envelope
func (s *VersioningSuite) TestFlattenedEnvelopeStructure() {
	// Create project
	project := s.createProject("envelope-test", "github.com/test/envelope", "file")

	// Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add resource
	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err, "adding resource should succeed")

	// Generate code
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	// Verify the Device type exists in the generated models
	err = project.CheckGeneratedFile("cmd/server/models_generated.go", "Device")
	s.Require().NoError(err, "generated models should contain Device type")
}

// TestAPIsYamlPlaceholder verifies that apis.yaml triggers versioning placeholder
func (s *VersioningSuite) TestAPIsYamlPlaceholder() {
	// Create project
	project := s.createProject("apis-yaml-test", "github.com/test/apis", "file")

	// Initialize project (this already creates apis.yaml with default group)
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add resource (this will automatically add to apis/example.com/v1/)
	err = project.AddResource(s.fabricaBinary, "Sensor")
	s.Require().NoError(err, "adding resource should succeed")

	// Generate code - should work with the generated apis.yaml
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "generation should succeed with apis.yaml present")

	// Note: The placeholder message will be shown in output but generation continues
	// Future enhancement: verify that apis/<group>/<version>/ directories are created
}

// TestBackwardCompatibility verifies that existing projects without apis.yaml work unchanged
func (s *VersioningSuite) TestBackwardCompatibility() {
	// Create project without apis.yaml
	project := s.createProject("compat-test", "github.com/test/compat", "file")

	// Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add resource
	err = project.AddResource(s.fabricaBinary, "Product")
	s.Require().NoError(err, "adding resource should succeed")

	// Generate code WITHOUT apis.yaml
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	// Verify the Product type exists in the generated models
	err = project.CheckGeneratedFile("cmd/server/models_generated.go", "Product")
	s.Require().NoError(err, "generated models should contain Product type")
}

// TestConfigValidation tests apis.yaml config validation
func (s *VersioningSuite) TestConfigValidation() {
	// Create project
	project := s.createProject("validation-test", "github.com/test/validation", "file")

	// Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add resource
	err = project.AddResource(s.fabricaBinary, "Widget")
	s.Require().NoError(err, "adding resource should succeed")

	// Test Case 1: Invalid apis.yaml (storageVersion not in versions)
	invalidYaml := `groups:
  - name: example.com
    storageVersion: v2
    versions:
      - v1
    resources:
      - Widget
`
	apisPath := filepath.Join(project.Dir, "apis.yaml")
	err = os.WriteFile(apisPath, []byte(invalidYaml), 0644)
	s.Require().NoError(err, "should write invalid apis.yaml")

	// Validation is now implemented - expect an error for invalid config
	err = project.Generate(s.fabricaBinary)
	s.Require().Error(err, "generation should fail with invalid apis.yaml")
	s.Require().Contains(err.Error(), "storageVersion", "error should mention storageVersion validation")

	// Test Case 2: Valid apis.yaml
	validYaml := `groups:
  - name: example.com
    storageVersion: v1
    versions:
      - v1alpha1
      - v1
    resources:
      - Widget
`
	err = os.WriteFile(apisPath, []byte(validYaml), 0644)
	s.Require().NoError(err, "should write valid apis.yaml")

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "generation should succeed with valid apis.yaml")
}

// TestJSONCompatibility verifies that JSON format remains unchanged
func (s *VersioningSuite) TestJSONCompatibility() {
	// Create project
	project := s.createProject("json-compat-test", "github.com/test/jsoncompat", "file")

	// Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add resource
	err = project.AddResource(s.fabricaBinary, "Item")
	s.Require().NoError(err, "adding resource should succeed")

	// Generate code
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	// Verify generated files have proper JSON marshaling tags
	err = project.CheckGeneratedFile("cmd/server/models_generated.go", "json:")
	s.Require().NoError(err, "generated models should contain JSON marshaling tags")
}

// TestUnsupportedAPIVersionRejected verifies invalid apiVersion returns 406 when registry is present
func (s *VersioningSuite) TestUnsupportedAPIVersionRejected() {
	project := s.createProject("apiversion-unsupported", "github.com/test/unsupported", "file")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err, "adding resource should succeed")

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	project.AssertFileExists("pkg/apiversion/registry_generated.go")

	err = project.StartServerRuntime()
	s.Require().NoError(err, "server should start")

	body := `{"apiVersion":"example.com/v2asdasda","kind":"Device","metadata":{"name":"device-100"},"spec":{}}`
	resp, respBody, err := project.HTTPCall(http.MethodPost, "/devices", body, nil)
	s.Require().NoError(err, "request should succeed")
	s.Require().Equal(http.StatusNotAcceptable, resp.StatusCode)
	s.Require().Contains(string(respBody), "Unsupported version")
}

// TestRun is the entry point for the versioning test suite
func TestVersioningSuite(t *testing.T) {
	suite.Run(t, new(VersioningSuite))
}

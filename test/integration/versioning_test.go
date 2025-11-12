// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/json"
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

	// Build to ensure it compiles
	err = project.Build()
	s.Require().NoError(err, "build should succeed with flattened envelope")

	// Verify generated code structure (check that models_generated.go exists)
	project.AssertFileExists("cmd/server/models_generated.go")

	// Start server and test JSON structure
	err = project.StartServer()
	s.Require().NoError(err, "server should start")
	defer project.StopServer() //nolint:all

	// Create a device and verify JSON has flattened structure
	spec := map[string]interface{}{
		"name":        "test-device",
		"description": "Test device for versioning",
	}

	resource, err := project.CreateResource("device", spec)
	s.Require().NoError(err, "should create resource")

	// Verify flattened envelope fields are present in JSON
	s.Require().Contains(resource, "apiVersion", "response should have apiVersion")
	s.Require().Contains(resource, "kind", "response should have kind")
	s.Require().Contains(resource, "metadata", "response should have metadata")
	s.Require().Contains(resource, "spec", "response should have spec")

	// Verify apiVersion has correct format
	apiVersion, ok := resource["apiVersion"].(string)
	s.Require().True(ok, "apiVersion should be string")
	s.Require().NotEmpty(apiVersion, "apiVersion should not be empty")

	// Verify kind matches resource name
	kind, ok := resource["kind"].(string)
	s.Require().True(ok, "kind should be string")
	s.Require().Equal("Device", kind, "kind should be Device")

	// Verify metadata structure
	metadata, ok := resource["metadata"].(map[string]interface{})
	s.Require().True(ok, "metadata should be object")
	s.Require().Contains(metadata, "name", "metadata should have name")
	s.Require().Contains(metadata, "uid", "metadata should have uid")
}

// TestAPIsYamlPlaceholder verifies that apis.yaml triggers versioning placeholder
func (s *VersioningSuite) TestAPIsYamlPlaceholder() {
	// Create project
	project := s.createProject("apis-yaml-test", "github.com/test/apis", "file")

	// Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add resource
	err = project.AddResource(s.fabricaBinary, "Sensor")
	s.Require().NoError(err, "adding resource should succeed")

	// Create apis.yaml to trigger versioning
	apisYaml := `groups:
  - name: test.example.io
    storageVersion: v1
    versions:
      - v1alpha1
      - v1beta1
      - v1
    resources:
      - kind: Sensor
`
	apisPath := filepath.Join(project.Dir, "apis.yaml")
	err = os.WriteFile(apisPath, []byte(apisYaml), 0644)
	s.Require().NoError(err, "should write apis.yaml")

	// Generate code - should show placeholder message
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

	// Build
	err = project.Build()
	s.Require().NoError(err, "build should succeed")

	// Start server
	err = project.StartServer()
	s.Require().NoError(err, "server should start")
	defer project.StopServer() //nolint:all

	// Create and retrieve resource - should work exactly as before
	spec := map[string]interface{}{
		"name": "test-product",
		"sku":  "TEST-001",
	}

	resource, err := project.CreateResource("product", spec)
	s.Require().NoError(err, "should create resource")

	// Verify basic structure still works
	s.Require().Contains(resource, "metadata", "should have metadata")
	s.Require().Contains(resource, "spec", "should have spec")
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
  - name: test.example.io
    storageVersion: v2
    versions:
      - v1
    resources:
      - kind: Widget
`
	apisPath := filepath.Join(project.Dir, "apis.yaml")
	err = os.WriteFile(apisPath, []byte(invalidYaml), 0644)
	s.Require().NoError(err, "should write invalid apis.yaml")

	// Note: Current implementation shows placeholder, validation would happen in Phase 2
	// For now, just ensure generation doesn't crash
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "generation should not crash with apis.yaml")

	// Test Case 2: Valid apis.yaml
	validYaml := `groups:
  - name: test.example.io
    storageVersion: v1
    versions:
      - v1alpha1
      - v1
    resources:
      - kind: Widget
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

	// Generate and build
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	err = project.Build()
	s.Require().NoError(err, "build should succeed")

	// Start server
	err = project.StartServer()
	s.Require().NoError(err, "server should start")
	defer project.StopServer() //nolint:all

	// Create resource
	spec := map[string]interface{}{
		"name":        "test-item",
		"description": "Test item",
	}

	resource, err := project.CreateResource("item", spec)
	s.Require().NoError(err, "should create resource")

	// Marshal to JSON and verify structure
	resourceJSON, err := json.Marshal(resource)
	s.Require().NoError(err, "should marshal to JSON")

	// Unmarshal back
	var parsed map[string]interface{}
	err = json.Unmarshal(resourceJSON, &parsed)
	s.Require().NoError(err, "should unmarshal JSON")

	// Verify all expected fields are present
	expectedFields := []string{"apiVersion", "kind", "metadata", "spec"}
	for _, field := range expectedFields {
		s.Require().Contains(parsed, field, "JSON should contain %s", field)
	}

	// Verify metadata subfields
	metadata, ok := parsed["metadata"].(map[string]interface{})
	s.Require().True(ok, "metadata should be object")

	expectedMetadataFields := []string{"name", "uid", "createdAt", "updatedAt"}
	for _, field := range expectedMetadataFields {
		s.Require().Contains(metadata, field, "metadata should contain %s", field)
	}
}

// TestRun is the entry point for the versioning test suite
func TestVersioningSuite(t *testing.T) {
	suite.Run(t, new(VersioningSuite))
}

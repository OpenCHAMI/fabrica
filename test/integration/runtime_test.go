// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// RuntimeTestSuite tests that generated code works at runtime with an actual server.
// Phase 2: Verify generated API servers start correctly and client library calls work.
type RuntimeTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

// SetupSuite initializes the test environment
func (s *RuntimeTestSuite) SetupSuite() {
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
func (s *RuntimeTestSuite) TearDownTest() {
	// Stop all servers
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

// Helper to create and track test projects
func (s *RuntimeTestSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

// TestServerStartupAndHealth verifies the generated server starts and responds to health checks
func (s *RuntimeTestSuite) TestServerStartupAndHealth() {
	project := s.createProject("startup-test", "github.com/test/startup", "file")

	// Initialize project
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	// Add a simple resource
	err = project.AddResource(s.fabricaBinary, "Thing")
	s.Require().NoError(err)

	// Generate code
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Start server
	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Verify health endpoint
	resp, err := http.Get(fmt.Sprintf("%s/health", project.ServerURL))
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestCRUDViaHTTP tests basic CRUD operations against running server
// This validates the full request/response cycle: HTTP layer, handlers, storage
func (s *RuntimeTestSuite) TestCRUDViaHTTP() {
	project := s.createProject("crud-http-test", "github.com/test/crud-http", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Create a resource
	// Note: API expects name at top level, not nested in metadata
	createPayload := map[string]interface{}{
		"name":        "test-device",
		"description": "A test device",
	}

	resp, body, err := project.HTTPCall("POST", "/devices", createPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, fmt.Sprintf("Create failed: %s", string(body)))

	var created map[string]interface{}
	err = json.Unmarshal(body, &created)
	s.Require().NoError(err)

	// Extract UID
	metadata, ok := created["metadata"].(map[string]interface{})
	s.Require().True(ok, "Response should have metadata")
	uid, ok := metadata["uid"].(string)
	s.Require().True(ok && uid != "", "Response should have uid")

	// List resources
	resp, body, err = project.HTTPCall("GET", "/devices", nil, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var devices []map[string]interface{}
	err = json.Unmarshal(body, &devices)
	s.Require().NoError(err)
	s.Require().Len(devices, 1, "Should have one device")

	// Get resource by UID
	resp, body, err = project.HTTPCall("GET", fmt.Sprintf("/devices/%s", uid), nil, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var retrieved map[string]interface{}
	err = json.Unmarshal(body, &retrieved)
	s.Require().NoError(err)

	retrievedMetadata, ok := retrieved["metadata"].(map[string]interface{})
	s.Require().True(ok, "Retrieved response should have metadata")
	s.Require().Equal("test-device", retrievedMetadata["name"])

	// Update resource
	// Note: API expects name and spec fields at top level
	updatePayload := map[string]interface{}{
		"name":        "test-device",
		"description": "Updated description",
	}

	resp, body, err = project.HTTPCall("PUT", fmt.Sprintf("/devices/%s", uid), updatePayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode, fmt.Sprintf("Update failed: %s", string(body)))

	// Verify update
	resp, body, err = project.HTTPCall("GET", fmt.Sprintf("/devices/%s", uid), nil, nil)
	s.Require().NoError(err)
	var updated map[string]interface{}
	err = json.Unmarshal(body, &updated)
	s.Require().NoError(err)

	updatedSpec := updated["spec"].(map[string]interface{})
	s.Require().Equal("Updated description", updatedSpec["description"])

	// Delete resource
	resp, body, err = project.HTTPCall("DELETE", fmt.Sprintf("/devices/%s", uid), nil, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode, fmt.Sprintf("Delete failed: %s", string(body)))

	// Verify deletion
	resp, _, err = project.HTTPCall("GET", fmt.Sprintf("/devices/%s", uid), nil, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

// TestMultiResourceProject verifies servers handle multiple resources correctly
func (s *RuntimeTestSuite) TestMultiResourceProject() {
	project := s.createProject("multi-resource-test", "github.com/test/multi", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Node")
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Rack")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Create Node
	nodePayload := map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "Node",
		"metadata": map[string]interface{}{
			"name": "node-1",
		},
		"spec": map[string]interface{}{
			"description": "Test node",
		},
	}

	resp, _, err := project.HTTPCall("POST", "/nodes", nodePayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	// Create Rack
	rackPayload := map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "Rack",
		"metadata": map[string]interface{}{
			"name": "rack-1",
		},
		"spec": map[string]interface{}{
			"description": "Test rack",
		},
	}

	resp, _, err = project.HTTPCall("POST", "/racks", rackPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	// List nodes
	resp, body, err := project.HTTPCall("GET", "/nodes", nil, nil)
	s.Require().NoError(err)
	var nodes []map[string]interface{}
	err = json.Unmarshal(body, &nodes)
	s.Require().NoError(err)
	s.Require().Len(nodes, 1)

	// List racks
	resp, body, err = project.HTTPCall("GET", "/racks", nil, nil)
	s.Require().NoError(err)
	var racks []map[string]interface{}
	err = json.Unmarshal(body, &racks)
	s.Require().NoError(err)
	s.Require().Len(racks, 1)
}

// TestPatchOperations verifies PATCH support in generated servers
func (s *RuntimeTestSuite) TestPatchOperations() {
	project := s.createProject("patch-test", "github.com/test/patch", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Config")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Create a config
	// Note: API expects name and spec fields at top level
	createPayload := map[string]interface{}{
		"name":        "my-config",
		"description": "Initial description",
	}

	resp, body, err := project.HTTPCall("POST", "/configs", createPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var created map[string]interface{}
	err = json.Unmarshal(body, &created)
	s.Require().NoError(err)

	metadata := created["metadata"].(map[string]interface{})
	uid := metadata["uid"].(string)

	// PATCH with strategic merge (update only description)
	// Note: Patch operates on spec fields at top level
	patchPayload := map[string]interface{}{
		"description": "Patched description",
	}

	resp, body, err = project.HTTPCall("PATCH", fmt.Sprintf("/configs/%s", uid), patchPayload, map[string]string{
		"Content-Type": "application/merge-patch+json",
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode, fmt.Sprintf("Patch failed: %s", string(body)))

	// Verify patch worked
	var patched map[string]interface{}
	err = json.Unmarshal(body, &patched)
	s.Require().NoError(err)
	spec := patched["spec"].(map[string]interface{})
	s.Require().Equal("Patched description", spec["description"])
}

// TestFileStorage specifically validates file-based storage backend
func (s *RuntimeTestSuite) TestFileStorage() {
	project := s.createProject("file-storage-test", "github.com/test/file-storage", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Secret")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Create a secret
	createPayload := map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name": "my-secret",
		},
		"spec": map[string]interface{}{
			"description": "A secret value",
		},
	}

	resp, _, err := project.HTTPCall("POST", "/secrets", createPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	// Verify file was created on disk
	dataDir := filepath.Join(project.Dir, "data")
	_, err = os.Stat(dataDir)
	s.Require().NoError(err, "Data directory should be created")

	// Verify secret file exists
	secretsDir := filepath.Join(dataDir, "secrets")
	entries, err := os.ReadDir(secretsDir)
	s.Require().NoError(err, "Secrets directory should exist")
	s.Require().Greater(len(entries), 0, "At least one secret file should exist")
}

// TestErrorHandling verifies appropriate error responses for invalid operations
func (s *RuntimeTestSuite) TestErrorHandling() {
	project := s.createProject("error-test", "github.com/test/errors", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Item")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Try to get non-existent resource
	resp, _, err := project.HTTPCall("GET", "/items/nonexistent", nil, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusNotFound, resp.StatusCode)

	// Try to delete non-existent resource
	resp, _, err = project.HTTPCall("DELETE", "/items/nonexistent", nil, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusNotFound, resp.StatusCode)

	// Try invalid method
	resp, _, err = project.HTTPCall("INVALID", "/items", nil, nil)
	// HTTP client allows any method string; server responds with 405
	s.Require().NoError(err)
	s.Require().Equal(http.StatusMethodNotAllowed, resp.StatusCode)
}

// TestOpenAPIGeneration verifies OpenAPI spec is generated and accessible
func (s *RuntimeTestSuite) TestOpenAPIGeneration() {
	project := s.createProject("openapi-test", "github.com/test/openapi", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "API")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Verify OpenAPI file was generated
	openAPIPath := filepath.Join(project.Dir, "cmd", "server", "openapi_generated.go")
	_, err = os.Stat(openAPIPath)
	s.Require().NoError(err, "OpenAPI file should be generated")

	// Verify it contains API paths
	content, err := os.ReadFile(openAPIPath)
	s.Require().NoError(err)
	s.Require().Contains(string(content), "/apis", "OpenAPI should define API paths")
}

// Run the test suite
func TestRuntimeSuite(t *testing.T) {
	suite.Run(t, new(RuntimeTestSuite))
}

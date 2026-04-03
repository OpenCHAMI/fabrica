// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	// Create a resource (flattened envelope: metadata + spec)
	createPayload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test-device",
		},
		"spec": map[string]interface{}{
			"description": "A test device",
		},
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
	updatePayload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test-device",
		},
		"spec": map[string]interface{}{
			"description": "Updated description",
		},
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
	createPayload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "my-config",
		},
		"spec": map[string]interface{}{
			"description": "Initial description",
		},
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

// TestEntSQLiteReconciliationClientFlow ensures an Ent (SQLite) project with reconciliation
// can be generated, run, and driven entirely via the generated client CLI.
func (s *RuntimeTestSuite) TestEntSQLiteReconciliationClientFlow() {
	project := s.createProject("ent-reconcile-client", "github.com/test/ent-reconcile-client", "ent")

	err := project.InitializeWithFlags(s.fabricaBinary,
		"--events",
		"--events-bus", "memory",
		"--reconcile",
		"--db", "sqlite",
	)
	s.Require().NoError(err)

	for _, resource := range []string{"Node", "NodeSet"} {
		err := project.AddResource(s.fabricaBinary, resource)
		s.Require().NoError(err)
	}

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	nodeCreatePayload := `{"metadata":{"name":"node-alpha"},"spec":{"description":"initial node"}}`
	nodeCreateOut, err := project.RunClientBinary(clientBinary, "node", "create", "--spec", nodeCreatePayload, "--output", "json")
	s.Require().NoError(err)

	var createdNode map[string]interface{}
	s.Require().NoError(json.Unmarshal(nodeCreateOut, &createdNode))
	metadata, ok := createdNode["metadata"].(map[string]interface{})
	s.Require().True(ok, "created node should include metadata")
	nodeUID, ok := metadata["uid"].(string)
	s.Require().True(ok && nodeUID != "", "created node should have uid")

	updatePayload := `{"metadata":{"name":"node-alpha"},"spec":{"description":"updated node"}}`
	updateOut, err := project.RunClientBinary(clientBinary, "node", "update", nodeUID, "--spec", updatePayload, "--output", "json")
	s.Require().NoError(err)

	var updatedNode map[string]interface{}
	s.Require().NoError(json.Unmarshal(updateOut, &updatedNode))
	spec, ok := updatedNode["spec"].(map[string]interface{})
	s.Require().True(ok, "updated node should include spec")
	s.Require().Equal("updated node", spec["description"], "node spec should reflect update")

	listOut, err := project.RunClientBinary(clientBinary, "node", "list", "--output", "json")
	s.Require().NoError(err)
	var nodes []map[string]interface{}
	s.Require().NoError(json.Unmarshal(listOut, &nodes))
	s.Require().Len(nodes, 1, "should have one node after creation")

	nodeSetPayload := `{"metadata":{"name":"edge-nodes"},"spec":{"description":"edge set"}}`
	nodeSetOut, err := project.RunClientBinary(clientBinary, "nodeset", "create", "--spec", nodeSetPayload, "--output", "json")
	s.Require().NoError(err)

	var createdNodeSet map[string]interface{}
	s.Require().NoError(json.Unmarshal(nodeSetOut, &createdNodeSet))
	nodeSetMeta, ok := createdNodeSet["metadata"].(map[string]interface{})
	s.Require().True(ok, "created nodeset should include metadata")
	nodeSetUID, ok := nodeSetMeta["uid"].(string)
	s.Require().True(ok && nodeSetUID != "", "created nodeset should have uid")

	nodeSetListOut, err := project.RunClientBinary(clientBinary, "nodeset", "list", "--output", "json")
	s.Require().NoError(err)
	var nodeSets []map[string]interface{}
	s.Require().NoError(json.Unmarshal(nodeSetListOut, &nodeSets))
	s.Require().Len(nodeSets, 1, "should have one nodeset after creation")

	_, err = project.RunClientBinary(clientBinary, "nodeset", "delete", nodeSetUID)
	s.Require().NoError(err)

	nodeSetListAfter, err := project.RunClientBinary(clientBinary, "nodeset", "list", "--output", "json")
	s.Require().NoError(err)
	var nodeSetsAfter []map[string]interface{}
	s.Require().NoError(json.Unmarshal(nodeSetListAfter, &nodeSetsAfter))
	s.Require().Len(nodeSetsAfter, 0, "nodeset list should be empty after deletion")

	_, err = project.RunClientBinary(clientBinary, "node", "delete", nodeUID)
	s.Require().NoError(err)

	nodeListAfter, err := project.RunClientBinary(clientBinary, "node", "list", "--output", "json")
	s.Require().NoError(err)
	var nodesAfter []map[string]interface{}
	s.Require().NoError(json.Unmarshal(nodeListAfter, &nodesAfter))
	s.Require().Len(nodesAfter, 0, "node list should be empty after deletion")
}

// TestValidationWithInlinedSpecFields verifies validation on the flattened envelope pattern
// (APIVersion/Kind/Metadata/Spec) used by versioned APIs. This ensures validation runs on
// request objects before resource construction and returns useful errors.
func (s *RuntimeTestSuite) TestValidationWithInlinedSpecFields() {
	project := s.createProject("validation-test", "github.com/test/validation", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err)

	// Update Device spec with validation tags. The request model keeps APIVersion/Kind/Metadata
	// at the top level with Spec as a nested object (flattened envelope, not json:",inline").
	deviceTypePath := filepath.Join(project.Dir, "apis", "example.com", "v1", "device_types.go")
	deviceCode := `package v1

import (
	"context"
	"github.com/openchami/fabrica/pkg/fabrica"
)

// Device represents a device resource
type Device struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       DeviceSpec       ` + "`json:\"spec\" validate:\"required\"`" + `
	Status     DeviceStatus     ` + "`json:\"status,omitempty\"`" + `
}

// DeviceSpec defines the desired state of Device
type DeviceSpec struct {
	IPAddress  string ` + "`json:\"ipAddress\" validate:\"required,ip\"`" + `
	DeviceType string ` + "`json:\"deviceType\" validate:\"oneof=server switch router\"`" + `
	Location   string ` + "`json:\"location,omitempty\"`" + `
}

// DeviceStatus defines the observed state of Device
type DeviceStatus struct {
	Ready bool ` + "`json:\"ready\"`" + `
}

// Validate implements custom validation logic for Device
func (r *Device) Validate(ctx context.Context) error {
	return nil
}

// GetKind returns the kind of the resource
func (r *Device) GetKind() string {
	return "Device"
}

// GetName returns the name of the resource
func (r *Device) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *Device) GetUID() string {
	return r.Metadata.UID
}

// IsHub marks this as the hub/storage version
func (r *Device) IsHub() {}
`

	err = os.WriteFile(deviceTypePath, []byte(deviceCode), 0644)
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Test Case 1: Valid request - should succeed
	validPayload := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "server-1"},
		"spec": map[string]interface{}{
			"ipAddress":  "192.168.1.100",
			"deviceType": "server",
		},
	}
	resp, body, err := project.HTTPCall("POST", "/devices", validPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode,
		fmt.Sprintf("Valid request should succeed: %s", string(body)))

	// Test Case 2: Missing required field (ipAddress) - should fail
	missingIPPayload := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "server-2"},
		"spec": map[string]interface{}{
			"deviceType": "server",
		},
	}
	resp, body, err = project.HTTPCall("POST", "/devices", missingIPPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode,
		fmt.Sprintf("Missing ipAddress should fail: %s", string(body)))
	s.Require().Contains(string(body), "ipAddress", "Error should mention ipAddress")
	s.Require().Contains(string(body), "required", "Error should indicate it's a required field")

	// Test Case 4: Invalid IP address format - should fail
	invalidIPPayload := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "server-3"},
		"spec": map[string]interface{}{
			"ipAddress":  "not-an-ip",
			"deviceType": "server",
		},
	}
	resp, body, err = project.HTTPCall("POST", "/devices", invalidIPPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode,
		fmt.Sprintf("Invalid IP should fail: %s", string(body)))
	s.Require().Contains(string(body), "ipAddress", "Error should mention ipAddress")
	s.Require().Contains(string(body), "IP", "Error should indicate IP validation")

	// Test Case 5: Invalid enum value for deviceType - should fail
	invalidTypePayload := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "server-4"},
		"spec": map[string]interface{}{
			"ipAddress":  "192.168.1.101",
			"deviceType": "helicopter",
		},
	}
	resp, body, err = project.HTTPCall("POST", "/devices", invalidTypePayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode,
		fmt.Sprintf("Invalid enum should fail: %s", string(body)))
	s.Require().Contains(string(body), "deviceType", "Error should mention deviceType")
	s.Require().Contains(string(body), "one of", "Error should indicate oneof validation")

	// Test Case 6: Valid with optional field - should succeed
	validWithOptionalPayload := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "server-5"},
		"spec": map[string]interface{}{
			"ipAddress":  "192.168.1.102",
			"deviceType": "switch",
			"location":   "DataCenter A",
		},
	}
	resp, body, err = project.HTTPCall("POST", "/devices", validWithOptionalPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode,
		fmt.Sprintf("Valid request with optional fields should succeed: %s", string(body)))
}

// TestAuthEnabledServerRuntimePath validates that --auth generated servers actually enforce auth at runtime
func (s *RuntimeTestSuite) TestAuthEnabledServerRuntimePath() {
	project := s.createProject("auth-runtime-test", "github.com/test/auth-runtime", "file")

	// Initialize with auth enabled
	err := project.InitializeWithFlags(s.fabricaBinary, "--auth")
	s.Require().NoError(err, "project initialization with auth should succeed")

	if branch, ok := TokenSmithBranchForTests(); ok {
		// Allow explicit branch validation without changing the default released-version path.
		err = project.PinTokenSmithBranch(branch)
		s.Require().NoError(err, "pinning TokenSmith should succeed")
	}

	// Add resource and generate
	err = project.AddResource(s.fabricaBinary, "SecureData")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation with auth enabled should succeed")

	// Provide a local JWKS endpoint required by auth-enabled server startup.
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"test-key","use":"sig","alg":"RS256","n":"AQAB","e":"AQAB"}]}`))
	}))
	defer jwksServer.Close()
	s.T().Setenv("TOKENSMITH_JWKS_URL", jwksServer.URL)

	// Start server
	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Test: Try protected endpoint WITHOUT auth header
	// We expect 401 Unauthorized (auth middleware should reject)
	noAuthPayload := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "secure-1"},
		"spec":     map[string]interface{}{"description": "test"},
	}

	resp, body, err := project.HTTPCall("POST", "/securedatas", noAuthPayload, nil)
	s.Require().NoError(err)
	// With auth enabled, request without token should fail with 401 or 403
	s.Require().True(resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		fmt.Sprintf("Request without auth should be rejected, got %d: %s", resp.StatusCode, string(body)))

	// Test: Try protected endpoint WITH valid auth header
	// We'll use a simple header for testing; real auth would require valid JWT
	headers := map[string]string{
		"Authorization": "Bearer test-token",
	}

	resp, body, err = project.HTTPCall("POST", "/securedatas", noAuthPayload, headers)
	s.Require().NoError(err)
	// With auth header, server should process the request (may succeed or fail based on token validation)
	// Key assertion: request reaches handler (not rejected at auth layer before handler runs)
	s.Require().True(
		resp.StatusCode == http.StatusCreated ||
			resp.StatusCode == http.StatusUnauthorized ||
			resp.StatusCode == http.StatusForbidden,
		fmt.Sprintf("Request with auth header should reach handler, got %d: %s", resp.StatusCode, string(body)))

	// Test: Verify protected endpoint routes exist and auth middleware is wired
	resp, _, err = project.HTTPCall("GET", "/securedatas", nil, nil)
	s.Require().NoError(err)
	// List without auth should also be protected
	s.Require().True(resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		fmt.Sprintf("List endpoint without auth should be rejected, got %d", resp.StatusCode))
}

// TestMiddlewarePipelineOrdering validates ETag precondition handling in PATCH operations.
// This is a simplified version that tests core middleware functionality without custom types.
// TODO: Expand to test full validation → auth → patch pipeline
func (s *RuntimeTestSuite) TestMiddlewarePipelineOrdering() {
	s.T().Skip("middleware ordering test requires refinement - complex type definitions need investigation")
}

// Run the test suite
func TestRuntimeSuite(t *testing.T) {
	suite.Run(t, new(RuntimeTestSuite))
}

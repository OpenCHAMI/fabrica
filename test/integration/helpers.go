// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// TestProject represents a fabrica test project
type TestProject struct {
	Name      string
	Dir       string
	Module    string
	Storage   string
	Resources []string
	serverCmd *exec.Cmd
	suite     *suite.Suite
}

// NewTestProject creates a new test project instance
func NewTestProject(s *suite.Suite, tempDir, name, module, storage string) *TestProject {
	return &TestProject{
		Name:    name,
		Dir:     filepath.Join(tempDir, name),
		Module:  module,
		Storage: storage,
		suite:   s,
	}
}

// Initialize creates and initializes the fabrica project
func (p *TestProject) Initialize(fabricaBinary string) error {
	cmd := exec.Command(fabricaBinary, "init", p.Name, "--module", p.Module, "--storage-type", p.Storage, "--storage")
	cmd.Dir = filepath.Dir(p.Dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fabrica init failed: %w\nOutput: %s", err, output)
	}

	// Add replace directive for local development with absolute path
	goModPath := filepath.Join(p.Dir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}

	// Get absolute path to fabrica project root
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	fabricaRoot := filepath.Join(wd, "..", "..")
	fabricaRootAbs, err := filepath.Abs(fabricaRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path to fabrica root: %w", err)
	}

	newContent := string(content) + fmt.Sprintf("\nreplace github.com/alexlovelltroy/fabrica => %s\n", fabricaRootAbs)
	err = os.WriteFile(goModPath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to update go.mod: %w", err)
	}

	// Add the fabrica module as a requirement after adding replace directive
	// Use go get with -d flag to download without building
	getCmd := exec.Command("go", "get", "-d", "github.com/alexlovelltroy/fabrica")
	getCmd.Dir = p.Dir
	getOutput, err := getCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get fabrica module: %w\nOutput: %s", err, getOutput)
	}

	// Run go mod tidy to resolve all transitive dependencies
	// This is important to ensure go.sum has entries for fabrica's dependencies
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = p.Dir
	_, tidyErr := tidyCmd.CombinedOutput()
	if tidyErr != nil {
		// If tidy fails, try to download all modules and tidy again
		fmt.Printf("⚠️  First go mod tidy failed, trying download and retry...\n")
		downloadCmd := exec.Command("go", "mod", "download", "all")
		downloadCmd.Dir = p.Dir
		if _, downloadErr := downloadCmd.CombinedOutput(); downloadErr == nil {
			// Try tidy one more time after download
			tidyCmd2 := exec.Command("go", "mod", "tidy")
			tidyCmd2.Dir = p.Dir
			if tidy2Output, tidy2Err := tidyCmd2.CombinedOutput(); tidy2Err != nil {
				fmt.Printf("⚠️  Warning: go mod tidy still failed after download: %s\n", string(tidy2Output))
			}
		}
	}

	return nil
}

// AddResource adds a resource to the project
func (p *TestProject) AddResource(fabricaBinary, resourceName string) error {
	cmd := exec.Command(fabricaBinary, "add", "resource", resourceName)
	cmd.Dir = p.Dir // Set working directory instead of using -C flag
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add resource %s: %w\nOutput: %s", resourceName, err, output)
	}

	p.Resources = append(p.Resources, resourceName)
	return nil
}

// Generate runs fabrica generate
func (p *TestProject) Generate(fabricaBinary string) error {
	cmd := exec.Command(fabricaBinary, "generate", "--storage", "--openapi", "--handlers", "--client")
	cmd.Dir = p.Dir // Set working directory instead of using -C flag
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fabrica generate failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// Build builds the server and client binaries
func (p *TestProject) Build() error {
	// Go mod tidy - if it fails, try go mod download as fallback
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = p.Dir
	tidyOutput, tidyErr := cmd.CombinedOutput()
	if tidyErr != nil {
		// Try go mod download as fallback
		fmt.Printf("⚠️  go mod tidy failed in %s, trying go mod download...\n", p.Dir)
		fmt.Printf("go mod tidy error: %s\n", string(tidyOutput))

		downloadCmd := exec.Command("go", "mod", "download")
		downloadCmd.Dir = p.Dir
		downloadOutput, downloadErr := downloadCmd.CombinedOutput()
		if downloadErr != nil {
			return fmt.Errorf("failed to tidy dependencies: %w\ngo mod tidy output: %s\ngo mod download output: %s",
				tidyErr, tidyOutput, downloadOutput)
		}
		fmt.Printf("✓ go mod download succeeded\n")
	}

	// Build server
	cmd = exec.Command("go", "build", "-o", "server", "./cmd/server")
	cmd.Dir = p.Dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("server build failed: %w\nOutput: %s", err, output)
	}

	// Build client
	cmd = exec.Command("go", "build", "-o", "client", "./cmd/client")
	cmd.Dir = p.Dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("client build failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// StartServer starts the generated server
func (p *TestProject) StartServer() error {
	if p.serverCmd != nil {
		return fmt.Errorf("server is already running")
	}

	p.serverCmd = exec.Command("./server")
	p.serverCmd.Dir = p.Dir

	err := p.serverCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for server to be ready
	for i := 0; i < 50; i++ { // Increased timeout
		resp, err := http.Get("http://localhost:8080/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close() //nolint:all
			return nil
		}
		if resp != nil {
			resp.Body.Close() //nolint:all
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("server failed to start within timeout")
}

// StopServer stops the running server
func (p *TestProject) StopServer() error {
	if p.serverCmd == nil {
		return nil
	}

	if err := p.serverCmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill server: %w", err)
	}

	p.serverCmd.Wait() //nolint:all Wait for process to exit
	p.serverCmd = nil
	return nil
}

// RunClient executes the generated client with given arguments
func (p *TestProject) RunClient(args ...string) ([]byte, error) {
	cmd := exec.Command("./client", args...)
	cmd.Dir = p.Dir
	return cmd.CombinedOutput()
}

// CreateResource creates a resource using the client
func (p *TestProject) CreateResource(resourceName string, spec interface{}) (map[string]interface{}, error) {
	var specJSON string
	if s, ok := spec.(string); ok {
		specJSON = s
	} else {
		specBytes, err := json.Marshal(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal spec: %w", err)
		}
		specJSON = string(specBytes)
	}

	output, err := p.RunClient(resourceName, "create", "--spec", specJSON)
	if err != nil {
		return nil, fmt.Errorf("create failed: %w\nOutput: %s", err, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse create response: %w\nOutput: %s", err, output)
	}

	return result, nil
}

// GetResource retrieves a resource by ID
func (p *TestProject) GetResource(resourceName, id string) (map[string]interface{}, error) {
	output, err := p.RunClient(resourceName, "get", id, "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("get failed: %w\nOutput: %s", err, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse get response: %w\nOutput: %s", err, output)
	}

	return result, nil
}

// ListResources lists all resources of a given type
func (p *TestProject) ListResources(resourceName string) ([]map[string]interface{}, error) {
	output, err := p.RunClient(resourceName, "list", "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("list failed: %w\nOutput: %s", err, output)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w\nOutput: %s", err, output)
	}

	return result, nil
}

// PatchResource patches a resource with given patch data
func (p *TestProject) PatchResource(resourceName, id string, patch interface{}) (map[string]interface{}, error) {
	var patchJSON string
	if s, ok := patch.(string); ok {
		patchJSON = s
	} else {
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal patch: %w", err)
		}
		patchJSON = string(patchBytes)
	}

	output, err := p.RunClient(resourceName, "patch", id, "--patch", patchJSON)
	if err != nil {
		return nil, fmt.Errorf("patch failed: %w\nOutput: %s", err, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse patch response: %w\nOutput: %s", err, output)
	}

	return result, nil
}

// DeleteResource deletes a resource by ID
func (p *TestProject) DeleteResource(resourceName, id string) error {
	output, err := p.RunClient(resourceName, "delete", id)
	if err != nil {
		return fmt.Errorf("delete failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// AssertFileExists verifies that a file exists in the project
func (p *TestProject) AssertFileExists(relativePath string) {
	fullPath := filepath.Join(p.Dir, relativePath)
	p.suite.Require().FileExists(fullPath, "File should exist: %s", relativePath)
}

// AssertResourceHasSpec verifies that a resource response has the expected spec values
func (p *TestProject) AssertResourceHasSpec(t require.TestingT, resource map[string]interface{}, expectedSpec map[string]interface{}) {
	spec, ok := resource["spec"].(map[string]interface{})
	require.True(t, ok, "resource should have spec field")

	for key, expectedValue := range expectedSpec {
		actualValue, exists := spec[key]
		require.True(t, exists, "spec should have key: %s", key)
		require.Equal(t, expectedValue, actualValue, "spec[%s] should match expected value", key)
	}
}

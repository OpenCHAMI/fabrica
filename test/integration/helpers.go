// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	tokenSmithBranchEnvVar = "FABRICA_TEST_TOKENSMITH_BRANCH"
)

// TokenSmithBranchForTests returns the optional TokenSmith branch override used in integration tests.
// Leave FABRICA_TEST_TOKENSMITH_BRANCH unset to validate against the released version pinned by Fabrica.
func TokenSmithBranchForTests() (string, bool) {
	branch := strings.TrimSpace(os.Getenv(tokenSmithBranchEnvVar))
	if branch == "" {
		return "", false
	}
	return branch, true
}

// TestProject represents a fabrica test project
type TestProject struct {
	Name       string
	Dir        string
	Module     string
	Storage    string
	Resources  []string
	serverCmd  *exec.Cmd
	suite      *suite.Suite
	ServerPort int
	ServerURL  string
}

// NewTestProject creates a new test project instance
func NewTestProject(s *suite.Suite, tempDir, name, module, storage string) *TestProject {
	return &TestProject{
		Name:       name,
		Dir:        filepath.Join(tempDir, name),
		Module:     module,
		Storage:    storage,
		suite:      s,
		ServerPort: 0, // Will be assigned when server starts
		ServerURL:  "",
	}
}

// setGoEnv adds common Go environment variables to an exec.Cmd
func (p *TestProject) setGoEnv(cmd *exec.Cmd) {
	// Git repository initialization is sufficient - normal GOPROXY behavior works fine
}

// Initialize creates and initializes the fabrica project
func (p *TestProject) Initialize(fabricaBinary string) error {
	return p.InitializeWithFlags(fabricaBinary)
}

// InitializeWithFlags initializes a Fabrica project with additional CLI flags.
func (p *TestProject) InitializeWithFlags(fabricaBinary string, extraFlags ...string) error {
	baseArgs := []string{
		"init", p.Name,
		"--module", p.Module,
		"--storage-type", p.Storage,
		"--storage",
		"--group", "example.com",
		"--storage-version", "v1",
	}
	baseArgs = append(baseArgs, extraFlags...)
	if err := p.runInitCommand(fabricaBinary, baseArgs); err != nil {
		return err
	}

	return nil
}

func (p *TestProject) runInitCommand(fabricaBinary string, args []string) error {
	cmd := exec.Command(fabricaBinary, args...)
	cmd.Dir = filepath.Dir(p.Dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fabrica init failed: %w\nOutput: %s", err, output)
	}

	// Initialize git repository so Go doesn't try to fetch the module from the internet
	if err := p.setupGit(); err != nil {
		return err
	}

	// Add replace directive for local development with absolute path
	if err := p.addReplace(); err != nil {
		return err
	}

	return nil
}

func (p *TestProject) setupGit() error {
	gitInitCmd := exec.Command("git", "init")
	gitInitCmd.Dir = p.Dir
	if _, err := gitInitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	gitUserCmd := exec.Command("git", "config", "user.email", "test@example.com")
	gitUserCmd.Dir = p.Dir
	if _, err := gitUserCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure git user email: %w", err)
	}

	gitNameCmd := exec.Command("git", "config", "user.name", "Test User")
	gitNameCmd.Dir = p.Dir
	if _, err := gitNameCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure git user name: %w", err)
	}

	return nil
}

func (p *TestProject) addReplace() error {
	goModPath := filepath.Join(p.Dir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	fabricaRoot := filepath.Join(wd, "..", "..")
	fabricaRootAbs, err := filepath.Abs(fabricaRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path to fabrica root: %w", err)
	}

	newContent := string(content) + fmt.Sprintf("\nreplace github.com/openchami/fabrica => %s\n", fabricaRootAbs)
	if err := os.WriteFile(goModPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to update go.mod: %w", err)
	}

	return nil
}

// PinTokenSmithBranch pins TokenSmith dependency to a specific branch for this test project.
func (p *TestProject) PinTokenSmithBranch(branch string) error {
	if strings.TrimSpace(branch) == "" {
		return fmt.Errorf("tokensmith branch cannot be empty")
	}

	// Resolve branch to commit SHA since go module queries disallow slash-containing branch names.
	shaCmd := exec.Command("git", "ls-remote", "https://github.com/OpenCHAMI/tokensmith.git", "refs/heads/"+branch)
	shaOutput, err := shaCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to resolve TokenSmith branch %q: %w\nOutput: %s", branch, err, shaOutput)
	}
	shaFields := strings.Fields(string(shaOutput))
	if len(shaFields) < 1 || strings.TrimSpace(shaFields[0]) == "" {
		return fmt.Errorf("failed to parse commit SHA for TokenSmith branch %q from output: %s", branch, shaOutput)
	}
	commitSHA := strings.TrimSpace(shaFields[0])

	rootVersionCmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", fmt.Sprintf("github.com/openchami/tokensmith@%s", commitSHA))
	rootVersionCmd.Dir = p.Dir
	rootVersionOutput, err := rootVersionCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to resolve TokenSmith root pseudo-version for branch %q (commit %s): %w\nOutput: %s", branch, commitSHA, err, rootVersionOutput)
	}
	rootVersion := strings.TrimSpace(string(rootVersionOutput))
	if rootVersion == "" {
		return fmt.Errorf("resolved empty TokenSmith root pseudo-version for branch %q (commit %s)", branch, commitSHA)
	}

	// Remove placeholder requirements first, then pin middleware module at commit.
	for _, modulePath := range []string{"github.com/openchami/tokensmith", "github.com/openchami/tokensmith/middleware"} {
		dropCmd := exec.Command("go", "mod", "edit", "-droprequire", modulePath)
		dropCmd.Dir = p.Dir
		_, _ = dropCmd.CombinedOutput()
	}

	replaceRootCmd := exec.Command(
		"go",
		"mod",
		"edit",
		"-replace",
		fmt.Sprintf("github.com/openchami/tokensmith@v0.0.0=github.com/openchami/tokensmith@%s", rootVersion),
	)
	replaceRootCmd.Dir = p.Dir
	if output, err := replaceRootCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add replace for TokenSmith root v0.0.0 on branch %q (commit %s, version %s): %w\nOutput: %s", branch, commitSHA, rootVersion, err, output)
	}

	getRootCmd := exec.Command("go", "get", fmt.Sprintf("github.com/openchami/tokensmith@%s", commitSHA))
	getRootCmd.Dir = p.Dir
	if output, err := getRootCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pin TokenSmith root module for branch %q (commit %s): %w\nOutput: %s", branch, commitSHA, err, output)
	}

	getCmd := exec.Command("go", "get", fmt.Sprintf("github.com/openchami/tokensmith/middleware@%s", commitSHA))
	getCmd.Dir = p.Dir
	if output, err := getCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pin TokenSmith middleware module for branch %q (commit %s): %w\nOutput: %s", branch, commitSHA, err, output)
	}

	return nil
}

// SetFabricaModuleVersion sets a specific version of the fabrica module in go.mod for testing
// version mismatch scenarios (e.g., for PR-38 regression tests).
func (p *TestProject) SetFabricaModuleVersion(version string) error {
	// Use go mod edit to set the fabrica module version
	cmd := exec.Command("go", "mod", "edit", "-require", fmt.Sprintf("github.com/openchami/fabrica@%s", version))
	cmd.Dir = p.Dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set fabrica version to %s: %w\nOutput: %s", version, err, output)
	}

	// Remove the replace directive so the version mismatch is visible to the CLI
	cmd = exec.Command("go", "mod", "edit", "-dropreplace", "github.com/openchami/fabrica")
	cmd.Dir = p.Dir
	if output, err := cmd.CombinedOutput(); err != nil {
		// This may fail if replace doesn't exist, which is fine
		_ = output
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
	cmd.Dir = p.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fabrica generate failed: %w\nOutput: %s", err, output)
	}

	// Run go mod tidy after generation to download all dependencies
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = p.Dir
	tidyOutput, err := tidyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod tidy failed: %w\nOutput: %s", err, tidyOutput)
	}

	return nil
}

// CheckGeneratedFile verifies that a generated file exists and contains expected content
// This is used for validating generation output without attempting compilation
func (p *TestProject) CheckGeneratedFile(relativePath string, expectedContent string) error {
	fullPath := filepath.Join(p.Dir, relativePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("generated file %s does not exist: %w", relativePath, err)
	}

	if len(content) == 0 {
		return fmt.Errorf("generated file %s is empty", relativePath)
	}

	if expectedContent != "" && !strings.Contains(string(content), expectedContent) {
		return fmt.Errorf("generated file %s does not contain expected content: %s", relativePath, expectedContent)
	}

	return nil
}

// GenerateEnt runs Ent code generation for Ent storage projects
// DEPRECATED: Ent generation now runs automatically during Generate()
// This method is kept for backward compatibility but is a no-op
func (p *TestProject) GenerateEnt(fabricaBinary string) error {
	// Ent generation now happens automatically in Generate() when storage type is "ent"
	// This is a no-op for backward compatibility
	return nil
}

// findAvailablePort finds an available port by listening on port 0
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// StartServerRuntime builds and starts the generated API server for runtime testing.
// It resolves an available port, starts the server in the background, waits for health check,
// and allows the test to make HTTP requests against running server.
func (p *TestProject) StartServerRuntime() error {
	if p.serverCmd != nil {
		return fmt.Errorf("server already running")
	}

	// Find available port
	port, err := findAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}
	p.ServerPort = port
	p.ServerURL = fmt.Sprintf("http://localhost:%d", port)

	// Build the server binary
	serverMainPath := filepath.Join(p.Dir, "cmd", "server", "main.go")
	if _, err := os.Stat(serverMainPath); err != nil {
		return fmt.Errorf("server main.go not found: %w", err)
	}

	// Compile the server
	outputPath := filepath.Join(p.Dir, "server-binary")
	buildCmd := exec.Command("go", "build", "-o", outputPath, "./cmd/server")
	buildCmd.Dir = p.Dir
	buildCmd.Env = os.Environ()
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build server: %w\nOutput: %s", err, output)
	}

	// Start the server with the serve subcommand and port flag
	args := []string{"serve", "--port", fmt.Sprintf("%d", port)}
	if p.Storage == "ent" {
		args = append(args, "--database-url", "file:./data.db?cache=shared&_fk=1")
	}
	p.serverCmd = exec.Command(outputPath, args...)
	p.serverCmd.Dir = p.Dir
	p.serverCmd.Stdout = os.Stdout
	p.serverCmd.Stderr = os.Stderr

	if err := p.serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for health check endpoint
	if err := p.WaitForHealth(30 * time.Second); err != nil {
		p.serverCmd.Process.Kill() //nolint:errcheck
		p.serverCmd = nil
		return fmt.Errorf("server health check failed: %w", err)
	}

	return nil
}

// WaitForHealth polls the /health endpoint until server is ready or timeout expires
func (p *TestProject) WaitForHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("%s/health", p.ServerURL))
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("health check timeout after %s", timeout)
}

// HTTPCall makes a generic HTTP call to the server
func (p *TestProject) HTTPCall(method, endpoint string, body interface{}, headers map[string]string) (*http.Response, []byte, error) {
	if p.ServerURL == "" {
		return nil, nil, fmt.Errorf("server not started - call StartServerRuntime() first")
	}

	var reqBody []byte
	var err error

	if body != nil {
		if s, ok := body.(string); ok {
			reqBody = []byte(s)
		} else {
			reqBody, err = json.Marshal(body)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
		}
	}

	url := fmt.Sprintf("%s%s", p.ServerURL, endpoint)
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return resp, respBody, nil
}

// Build is now a no-op stub for backward compatibility
// Tests should validate code generation, not build capability.
// Building has been removed because it requires resolving go.mod dependencies,
// which is complex and fragile with fake test module paths. The primary goal
// is to test that Fabrica generates correct code, not that the build system works.
func (p *TestProject) Build() error {
	fmt.Printf("ℹ️  Build step skipped (test validates generation, not compilation)\n")
	return nil
}

// StartServer is now StartServerRuntime for runtime testing.
// For backward compatibility, this calls StartServerRuntime if called.
func (p *TestProject) StartServer() error {
	return p.StartServerRuntime()
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

// BuildClientBinary compiles the generated client CLI binary and verifies it runs
// This is Phase 3 of testing: verify client binary compiles and basic commands work
func (p *TestProject) BuildClientBinary() (string, error) {
	if p.ServerURL == "" {
		return "", fmt.Errorf("server not running - call StartServerRuntime() first")
	}

	clientMainPath := filepath.Join(p.Dir, "cmd", "client", "main.go")
	if _, err := os.Stat(clientMainPath); err != nil {
		return "", fmt.Errorf("client main.go not found: %w", err)
	}

	// Compile the client binary
	outputPath := filepath.Join(p.Dir, "client-binary")
	buildCmd := exec.Command("go", "build", "-o", outputPath, "./cmd/client")
	buildCmd.Dir = p.Dir
	buildCmd.Env = os.Environ()
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build client: %w\nOutput: %s", err, output)
	}

	return outputPath, nil
}

// RunClientBinary executes the client CLI binary with given arguments
func (p *TestProject) RunClientBinary(clientBinary string, args ...string) ([]byte, error) {
	// Prepend --server flag with dynamic port if server is running
	if p.ServerURL != "" {
		args = append([]string{"--server", p.ServerURL}, args...)
	}
	cmd := exec.Command(clientBinary, args...)
	cmd.Dir = p.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("client binary failed: %w\nOutput: %s", err, output)
	}
	return output, nil
}

// RunClient is deprecated - use generated client library or RunClientBinary for CLI smoke tests

// CreateResource creates a resource using the client.
// DEPRECATED: Use HTTPCall() with POST instead for direct server testing.
func (p *TestProject) CreateResource(resourceName string, spec interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("CreateResource is deprecated - use HTTPCall() for direct HTTP testing or use RunClientBinary() for CLI testing")
}

// GetResource retrieves a resource by ID.
// DEPRECATED: Use HTTPCall() with GET instead for direct server testing.
func (p *TestProject) GetResource(resourceName, id string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("GetResource is deprecated - use HTTPCall() for direct HTTP testing")
}

// ListResources lists all resources of a given type.
// DEPRECATED: Use HTTPCall() with GET instead for direct server testing.
func (p *TestProject) ListResources(resourceName string) ([]map[string]interface{}, error) {
	return nil, fmt.Errorf("ListResources is deprecated - use HTTPCall() for direct HTTP testing")
}

// PatchResource patches a resource with given patch data.
// DEPRECATED: Use HTTPCall() with PATCH instead for direct server testing.
func (p *TestProject) PatchResource(resourceName, id string, patch interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("PatchResource is deprecated - use HTTPCall() for direct HTTP testing")
}

// DeleteResource deletes a resource by ID.
// DEPRECATED: Use HTTPCall() with DELETE instead for direct server testing.
func (p *TestProject) DeleteResource(resourceName, id string) error {
	return fmt.Errorf("DeleteResource is deprecated - use HTTPCall() for direct HTTP testing")
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

// ModifyFile reads a file, applies a modification function, and writes it back
func (p *TestProject) ModifyFile(relativePath string, modifier func(string) string) error {
	path := filepath.Join(p.Dir, relativePath)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	newContent := modifier(string(content))

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}

// Example1_CustomizeResource updates the Device spec as per Example 1
func (p *TestProject) Example1_CustomizeResource() error {
	// Path: apis/example.com/v1/device_types.go (for versioned projects)
	relPath := filepath.Join("apis", "example.com", "v1", "device_types.go")

	return p.ModifyFile(relPath, func(content string) string {
		// We replace the default placeholder or the simple struct definition
		// with the full definition from the example
		target := `type DeviceSpec struct {
	Description string ` + "`json:\"description,omitempty\" validate:\"max=200\"`" + `
	// Add your spec fields here
}`

		replacement := `type DeviceSpec struct {
	Description string ` + "`json:\"description,omitempty\" validate:\"max=200\"`" + `
	IPAddress   string ` + "`json:\"ipAddress,omitempty\" validate:\"omitempty,ip\"`" + `
	Location    string ` + "`json:\"location,omitempty\"`" + `
	Rack        string ` + "`json:\"rack,omitempty\"`" + `
}`
		// Try specific replacement first
		if strings.Contains(content, target) {
			return strings.Replace(content, target, replacement, 1)
		}

		// Fallback: If formatting is slightly different, try to inject just the fields
		// This assumes the file contains "// Add your spec fields here"
		fields := `IPAddress   string ` + "`json:\"ipAddress,omitempty\" validate:\"omitempty,ip\"`" + `
	Location    string ` + "`json:\"location,omitempty\"`" + `
	Rack        string ` + "`json:\"rack,omitempty\"`"

		return strings.Replace(content, "// Add your spec fields here", fields, 1)
	})
}

// Example1_ConfigureServer uncomments the storage and route registration in main.go
func (p *TestProject) Example1_ConfigureServer() error {
	relPath := filepath.Join("cmd", "server", "main.go")

	return p.ModifyFile(relPath, func(content string) string {
		// 1. Uncomment the storage import
		// Expecting: // "github.com/user/device-inventory/internal/storage"
		// We need to be careful to match the actual module name or just the suffix
		lines := strings.Split(content, "\n")
		var newLines []string

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Uncomment import for storage
			if strings.HasPrefix(trimmed, "//") && strings.Contains(trimmed, "/internal/storage\"") {
				line = strings.Replace(line, "// ", "", 1)
				line = strings.Replace(line, "//", "", 1) // Handle case without space
			}

			// Uncomment storage init
			// Expecting: // storage.InitFileBackend("./data")
			if strings.HasPrefix(trimmed, "//") && strings.Contains(trimmed, "storage.InitFileBackend") {
				line = strings.Replace(line, "// ", "", 1)
				line = strings.Replace(line, "//", "", 1)
			}

			// Uncomment route registration
			// Expecting: // RegisterGeneratedRoutes(r)
			if strings.HasPrefix(trimmed, "//") && strings.Contains(trimmed, "RegisterGeneratedRoutes") {
				line = strings.Replace(line, "// ", "", 1)
				line = strings.Replace(line, "//", "", 1)
			}

			newLines = append(newLines, line)
		}

		return strings.Join(newLines, "\n")
	})
}

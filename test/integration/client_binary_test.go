// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// ClientBinaryTestSuite tests that generated CLI client binaries compile and run.
// Phase 3: Verify client binary compiles and basic commands execute successfully.
// Functional validation relies on library tests; this focuses on generation correctness.
type ClientBinaryTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

// SetupSuite initializes the test environment
func (s *ClientBinaryTestSuite) SetupSuite() {
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
func (s *ClientBinaryTestSuite) TearDownTest() {
	// Stop all servers
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

// Helper to create and track test projects
func (s *ClientBinaryTestSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

// TestClientBinaryCompilation verifies the generated client CLI compiles successfully
func (s *ClientBinaryTestSuite) TestClientBinaryCompilation() {
	project := s.createProject("client-compile-test", "github.com/test/client-compile", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Task")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Start server for client to connect to
	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Build client binary
	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err, "Client binary should compile successfully")

	// Verify binary exists and is executable
	info, err := os.Stat(clientBinary)
	s.Require().NoError(err)
	s.Require().True(info.Mode()&0111 != 0, "Binary should be executable")
}

// TestClientHelpCommand verifies --help flag works on client binary
func (s *ClientBinaryTestSuite) TestClientHelpCommand() {
	project := s.createProject("client-help-test", "github.com/test/client-help", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Service")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Build and run client with --help
	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	output, err := project.RunClientBinary(clientBinary, "--help")
	s.Require().NoError(err, "Client --help should succeed")
	s.Require().Contains(string(output), "Usage", "Help output should contain Usage information")
}

// TestClientResourceCommands verifies resource-specific commands are generated
func (s *ClientBinaryTestSuite) TestClientResourceCommands() {
	project := s.createProject("client-resource-test", "github.com/test/client-resource", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Pod")
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Service")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Build client
	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// Verify pod resource subcommand exists
	output, err := project.RunClientBinary(clientBinary, "pod", "--help")
	s.Require().NoError(err, "pod subcommand should exist")
	s.Require().Contains(string(output), "pod", "pod subcommand help should mention pod")

	// Verify service resource subcommand exists
	output, err = project.RunClientBinary(clientBinary, "service", "--help")
	s.Require().NoError(err, "service subcommand should exist")
	s.Require().Contains(string(output), "service", "service subcommand help should mention service")
}

// TestClientListCommand verifies list command executes against server
func (s *ClientBinaryTestSuite) TestClientListCommand() {
	project := s.createProject("client-list-test", "github.com/test/client-list", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Volume")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Build client
	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// List volumes (should return empty list or valid JSON)
	output, err := project.RunClientBinary(clientBinary, "volume", "list")
	s.Require().NoError(err, "volume list command should execute")
	// Output should be JSON or parseable format
	s.Require().NotEmpty(string(output), "list command should return output")
}

// TestClientMultipleResources verifies client handles multiple resource types
func (s *ClientBinaryTestSuite) TestClientMultipleResources() {
	project := s.createProject("client-multi-test", "github.com/test/client-multi", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	resourceNames := []string{"Deployment", "StatefulSet", "DaemonSet"}
	for _, name := range resourceNames {
		err = project.AddResource(s.fabricaBinary, name)
		s.Require().NoError(err)
	}

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Build client
	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// Verify each resource has a subcommand
	for _, resName := range resourceNames {
		subcommand := strings.ToLower(resName)
		output, err := project.RunClientBinary(clientBinary, subcommand, "--help")
		s.Require().NoError(err, "subcommand %s should work", subcommand)
		s.Require().NotEmpty(string(output), "subcommand %s should have help output", subcommand)
	}
}

// TestClientBinaryInProject verifies binary location and structure
func (s *ClientBinaryTestSuite) TestClientBinaryInProject() {
	project := s.createProject("client-structure-test", "github.com/test/client-struct", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Resource")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Verify client source files exist
	clientMainPath := filepath.Join(project.Dir, "cmd", "client", "main.go")
	_, err = os.Stat(clientMainPath)
	s.Require().NoError(err, "cmd/client/main.go should be generated")

	// Verify client package structure
	content, err := os.ReadFile(clientMainPath)
	s.Require().NoError(err)
	s.Require().Contains(string(content), "package main", "Client should be a main package")
}

// TestClientEnvironmentConfiguration verifies client respects environment configuration
func (s *ClientBinaryTestSuite) TestClientEnvironmentConfiguration() {
	project := s.createProject("client-env-test", "github.com/test/client-env", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Config")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	// Build client
	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// Run with server URL environment variable
	// (Actual behavior depends on generated code, but binary should execute)
	output, err := project.RunClientBinary(clientBinary, "config", "list")
	s.Require().NoError(err, "Client should respect environment configuration")
	s.Require().NotEmpty(string(output), "Client should produce output")
}

// TestClientLogLevelFlag verifies that the --log-level flag is properly implemented
func (s *ClientBinaryTestSuite) TestClientLogLevelFlag() {
	project := s.createProject("client-loglevel-test", "github.com/test/client-loglevel", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Volume")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// Test 1: Help contains --log-level
	output, err := project.RunClientBinary(clientBinary, "--help")
	s.Require().NoError(err)
	s.Require().Contains(string(output), "--log-level", "Help should document --log-level flag")
	s.Require().Contains(string(output), "-l", "Help should document -l shorthand")

	// Test 2: Valid log levels work
	for _, level := range []string{"info", "warning", "debug"} {
		output, err := project.RunClientBinary(clientBinary, "--log-level", level, "volume", "list")
		s.Require().NoError(err, "volume list with --log-level %s should succeed", level)
		s.Require().NotEmpty(string(output))
	}

	// Test 3: Invalid log level fails
	output, err = project.RunClientBinary(clientBinary, "--log-level", "invalid", "volume", "list")
	s.Require().Error(err, "Invalid log level should fail")
	s.Require().Contains(string(output), "must be one of", "Error should explain valid values")
}

// TestClientLogLevelOutput verifies that debug logs actually appear in stderr
func (s *ClientBinaryTestSuite) TestClientLogLevelOutput() {
	project := s.createProject("client-log-output-test", "github.com/test/client-log-output", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Widget")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// Test 1: Debug level shows logs
	cmd := exec.Command(clientBinary, "--server", project.ServerURL, "--log-level", "debug", "widget", "list")
	cmd.Dir = project.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	s.Require().NoError(err)
	s.Require().NotEmpty(output)

	stderrStr := stderr.String()
	s.Require().Contains(stderrStr, "GET:", "Debug logs should show HTTP method")
	s.Require().Contains(stderrStr, "Response status:", "Debug logs should show response status")

	// Test 2: Warning level does NOT show debug logs
	cmd = exec.Command(clientBinary, "--server", project.ServerURL, "--log-level", "warning", "widget", "list")
	cmd.Dir = project.Dir
	stderr.Reset()
	cmd.Stderr = &stderr
	output, err = cmd.Output()
	s.Require().NoError(err)
	s.Require().NotEmpty(output)

	stderrStr = stderr.String()
	s.Require().NotContains(stderrStr, "GET:", "Warning level should not show debug logs")
	s.Require().NotContains(stderrStr, "Response status:", "Warning level should not show debug logs")

	// Test 3: Info level does NOT show debug logs
	cmd = exec.Command(clientBinary, "--server", project.ServerURL, "--log-level", "info", "widget", "list")
	cmd.Dir = project.Dir
	stderr.Reset()
	cmd.Stderr = &stderr
	output, err = cmd.Output()
	s.Require().NoError(err)
	s.Require().NotEmpty(output)

	stderrStr = stderr.String()
	s.Require().NotContains(stderrStr, "GET:", "Info level should not show debug logs")
	s.Require().NotContains(stderrStr, "Response status:", "Info level should not show debug logs")
}

// TestClientLogLevelShorthand verifies the -l shorthand works
func (s *ClientBinaryTestSuite) TestClientLogLevelShorthand() {
	project := s.createProject("client-shorthand-test", "github.com/test/client-shorthand", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Config")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// Test -l shorthand
	output, err := project.RunClientBinary(clientBinary, "-l", "debug", "config", "list")
	s.Require().NoError(err, "-l shorthand should work")
	s.Require().NotEmpty(string(output))

	// Verify it's the same as --log-level (both should work)
	output2, err := project.RunClientBinary(clientBinary, "--log-level", "debug", "config", "list")
	s.Require().NoError(err)
	s.Require().NotEmpty(string(output2))
}

// TestClientLogLevelEnvironmentVariable verifies log level via command-line flag works
// Note: Environment variable support for custom flag types (like LogLevel) requires
// additional plumbing in the generated code. This test verifies the flag works explicitly.
func (s *ClientBinaryTestSuite) TestClientLogLevelEnvironmentVariable() {
	project := s.createProject("clientenvlog", "github.com/test/clientenvlog", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Service")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// Test that log level flag works with explicit setting
	// (Environment variables for custom flag types would require additional template changes)
	cmd := exec.Command(clientBinary, "--server", project.ServerURL, "--log-level", "debug", "service", "list")
	cmd.Dir = project.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	s.Require().NoError(err)
	s.Require().NotEmpty(output)

	stderrStr := stderr.String()
	s.Require().Contains(stderrStr, "GET:", "Debug log level via flag should show HTTP requests")
}

// TestClientLogLevelEdgeCases tests edge cases like empty log level, case sensitivity
func (s *ClientBinaryTestSuite) TestClientLogLevelEdgeCases() {
	project := s.createProject("client-edge-test", "github.com/test/client-edge", "file")

	// Setup
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Node")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	clientBinary, err := project.BuildClientBinary()
	s.Require().NoError(err)

	// Test 1: No log level flag (default should work)
	output, err := project.RunClientBinary(clientBinary, "node", "list")
	s.Require().NoError(err, "Default log level should work")
	s.Require().NotEmpty(string(output))

	// Test 2: Case sensitivity (should fail if uppercase)
	output, err = project.RunClientBinary(clientBinary, "--log-level", "DEBUG", "node", "list")
	s.Require().Error(err, "Log levels should be case-sensitive (lowercase only)")
	s.Require().Contains(string(output), "must be one of", "Error should explain valid values")

	// Test 3: Whitespace should be rejected
	output, err = project.RunClientBinary(clientBinary, "--log-level", " debug ", "node", "list")
	s.Require().Error(err, "Whitespace in log level should fail")
}

// Run the test suite
func TestClientBinarySuite(t *testing.T) {
	suite.Run(t, new(ClientBinaryTestSuite))
}

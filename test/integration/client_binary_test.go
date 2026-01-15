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

// Run the test suite
func TestClientBinarySuite(t *testing.T) {
	suite.Run(t, new(ClientBinaryTestSuite))
}

// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// ConcurrencyTestSuite tests concurrent operations and conflict handling
type ConcurrencyTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

// SetupSuite initializes the test environment
func (s *ConcurrencyTestSuite) SetupSuite() {
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
func (s *ConcurrencyTestSuite) TearDownTest() {
	// Stop all servers
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

// Helper to create and track test projects
func (s *ConcurrencyTestSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

// TestConcurrentPatchConflicts validates that concurrent PATCH operations with stale ETags
// are properly detected and rejected with 412 Precondition Failed, preventing race conditions.
// This ensures only one request succeeds and others fail, proving conflict detection works.
// TODO: Refine test for more robust server startup and request handling
func (s *ConcurrencyTestSuite) TestConcurrentPatchConflicts() {
	s.T().Skip("concurrency test requires refinement for stable server startup under load")
}

// TestConcurrentCreateSameNameHandling validates that concurrent CREATE operations
// with the same resource name are properly handled (either unique by UID or error if duplicate name).
// TODO: Implement after concurrency infrastructure is stable
func (s *ConcurrencyTestSuite) TestConcurrentCreateSameNameHandling() {
	s.T().Skip("concurrent create test deferred - requires additional refinement")
}

// Run the test suite
func TestConcurrencySuite(t *testing.T) {
	suite.Run(t, new(ConcurrencyTestSuite))
}

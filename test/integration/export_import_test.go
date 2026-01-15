// SPDX-FileCopyrightText: 2026 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
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

// ExportImportTestSuite tests export/import command generation
type ExportImportTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

// SetupSuite initializes the test environment
func (s *ExportImportTestSuite) SetupSuite() {
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
func (s *ExportImportTestSuite) TearDownTest() {
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

// Helper to create and track test projects
func (s *ExportImportTestSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

// TestExportImportGeneration tests that export.go and import.go are generated correctly
func (s *ExportImportTestSuite) TestExportImportGeneration() {
	// Create project with Ent storage (export/import only work with Ent)
	project := s.createProject("export-import-test", "github.com/test/export", "ent")

	// Initialize project (creates apis/ structure)
	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err, "project initialization should succeed")

	// Add resources
	err = project.AddResource(s.fabricaBinary, "Server")
	s.Require().NoError(err, "adding Server resource should succeed")

	err = project.AddResource(s.fabricaBinary, "Node")
	s.Require().NoError(err, "adding Node resource should succeed")

	// Generate code
	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")

	// Verify export and import files exist
	exportFile := filepath.Join(project.Dir, "cmd", "server", "export.go")
	importFile := filepath.Join(project.Dir, "cmd", "server", "import.go")

	s.Require().FileExists(exportFile, "export.go should be generated")
	s.Require().FileExists(importFile, "import.go should be generated")

	// Verify import file uses UniqueImports, not hard-coded pkg/resources
	importContent, err := os.ReadFile(importFile)
	s.Require().NoError(err, "should read import.go")

	importStr := string(importContent)

	// Should NOT contain old-style hard-coded import paths
	s.Assert().NotContains(importStr, "pkg/resources/server",
		"import.go should not contain hard-coded pkg/resources/server import")
	s.Assert().NotContains(importStr, "pkg/resources/node",
		"import.go should not contain hard-coded pkg/resources/node import")

	// Should contain proper import section (verify structure exists)
	s.Assert().Contains(importStr, `"github.com/test/export/internal/storage"`,
		"import.go should import storage package")

	// Verify it imports from correct location (apis/)
	s.Assert().Contains(importStr, "github.com/test/export/apis/",
		"import.go should import from apis/ directory")

	// Verify export file structure
	exportContent, err := os.ReadFile(exportFile)
	s.Require().NoError(err, "should read export.go")

	exportStr := string(exportContent)

	// Verify export has correct imports
	s.Assert().Contains(exportStr, `"github.com/test/export/internal/storage"`,
		"export.go should import storage package")

	// Check that resource handling code exists
	s.Assert().Contains(importStr, `case "Server":`,
		"import.go should handle Server resource")
	s.Assert().Contains(importStr, `case "Node":`,
		"import.go should handle Node resource")
}

// TestExportImportUseCorrectTypes verifies that export/import use proper type references
func (s *ExportImportTestSuite) TestExportImportUseCorrectTypes() {
	project := s.createProject("type-test", "github.com/test/types", "ent")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	importFile := filepath.Join(project.Dir, "cmd", "server", "import.go")
	importContent, err := os.ReadFile(importFile)
	s.Require().NoError(err)

	importStr := string(importContent)

	// Should use PackageAlias.TypeName pattern, not toLower pattern
	// Looking for something like: var res v1.Device
	// NOT: var res device.Device
	s.Assert().Contains(importStr, "var res",
		"import.go should declare resource variables")

	// Should NOT use the old toLower pattern
	s.Assert().NotContains(importStr, ".Device | toLower",
		"import.go should not use toLower in type names")

	// Verify storage function calls are correct
	s.Assert().Contains(importStr, "storage.GetDeviceByUID",
		"import.go should use correct storage functions")
}

// TestExportImportFileBackend tests export/import with file storage backend
func (s *ExportImportTestSuite) TestExportImportFileBackend() {
	project := s.createProject("file-export", "github.com/test/fileexport", "file")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Config")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Verify export/import files are NOT generated for file backend
	// (they only work with Ent storage which has query methods)
	importFile := filepath.Join(project.Dir, "cmd", "server", "import.go")
	exportFile := filepath.Join(project.Dir, "cmd", "server", "export.go")

	_, importErr := os.Stat(importFile)
	_, exportErr := os.Stat(exportFile)

	s.Assert().True(os.IsNotExist(importErr), "import.go should NOT be generated for file storage")
	s.Assert().True(os.IsNotExist(exportErr), "export.go should NOT be generated for file storage")
}

// TestExportImportEntBackend tests export/import with Ent storage backend
func (s *ExportImportTestSuite) TestExportImportEntBackend() {
	project := s.createProject("ent-export", "github.com/test/entexport", "ent")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Entity")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Verify both files exist and use correct storage queries
	importFile := filepath.Join(project.Dir, "cmd", "server", "import.go")
	exportFile := filepath.Join(project.Dir, "cmd", "server", "export.go")

	s.Require().FileExists(importFile)
	s.Require().FileExists(exportFile)

	// Check for Ent-specific storage usage
	importContent, err := os.ReadFile(importFile)
	s.Require().NoError(err)
	s.Assert().Contains(string(importContent), "storage.GetEntityByUID",
		"import.go should use generated storage functions")

	exportContent, err := os.ReadFile(exportFile)
	s.Require().NoError(err)
	exportStr := string(exportContent)

	// Check for Ent-specific storage usage (case-insensitive for plural form)
	hasQuery := strings.Contains(strings.ToLower(exportStr), "storage.query") &&
		strings.Contains(strings.ToLower(exportStr), "entit")
	s.Assert().True(hasQuery,
		"export.go should use generated query functions for Entity resources")
}

// TestExportImportMultipleResources verifies export/import work with multiple resources
func (s *ExportImportTestSuite) TestExportImportMultipleResources() {
	project := s.createProject("multi-export", "github.com/test/multiexport", "ent")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	// Add multiple resources (avoid Go keywords like "switch")
	resources := []string{"Chassis", "Node", "BMC", "Router"}
	for _, res := range resources {
		err = project.AddResource(s.fabricaBinary, res)
		s.Require().NoError(err, "adding %s should succeed", res)
	}

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Verify import handles all resources
	importFile := filepath.Join(project.Dir, "cmd", "server", "import.go")
	importContent, err := os.ReadFile(importFile)
	s.Require().NoError(err)

	importStr := string(importContent)

	// Check each resource has a case statement
	for _, res := range resources {
		s.Assert().Contains(importStr, `case "`+res+`":`,
			"import.go should handle %s resource", res)
	}

	// Verify no duplicate imports
	lines := strings.Split(importStr, "\n")
	importsSeen := make(map[string]int)
	inImportBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "import (" {
			inImportBlock = true
			continue
		}
		if trimmed == ")" && inImportBlock {
			inImportBlock = false
			continue
		}
		if inImportBlock && strings.Contains(trimmed, "github.com/test/multiexport/apis/") {
			importsSeen[trimmed]++
		}
	}

	// Each import should appear exactly once
	for imp, count := range importsSeen {
		s.Assert().Equal(1, count, "Import %s should appear exactly once, found %d times", imp, count)
	}
}

// TestExportImportCommandFlags verifies command structure and flags
func (s *ExportImportTestSuite) TestExportImportCommandFlags() {
	project := s.createProject("flags-test", "github.com/test/flags", "ent")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Item")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	// Check export command structure
	exportFile := filepath.Join(project.Dir, "cmd", "server", "export.go")
	exportContent, err := os.ReadFile(exportFile)
	s.Require().NoError(err)

	exportStr := string(exportContent)
	s.Assert().Contains(exportStr, "newExportCommand()",
		"export.go should define newExportCommand")
	s.Assert().Contains(exportStr, "--format",
		"export should have format flag")
	s.Assert().Contains(exportStr, "--output",
		"export should have output flag")

	// Check import command structure
	importFile := filepath.Join(project.Dir, "cmd", "server", "import.go")
	importContent, err := os.ReadFile(importFile)
	s.Require().NoError(err)

	importStr := string(importContent)
	s.Assert().Contains(importStr, "newImportCommand()",
		"import.go should define newImportCommand")
	s.Assert().Contains(importStr, "--input",
		"import should have input flag")
	s.Assert().Contains(importStr, "--mode",
		"import should have mode flag")
	s.Assert().Contains(importStr, "--dry-run",
		"import should have dry-run flag")
}

func TestExportImportSuite(t *testing.T) {
	suite.Run(t, new(ExportImportTestSuite))
}

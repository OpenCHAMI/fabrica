// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type OTelTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

func (s *OTelTestSuite) SetupSuite() {
	wd, err := os.Getwd()
	s.Require().NoError(err)

	projectRoot := filepath.Join(wd, "..", "..")
	s.fabricaBinary = filepath.Join(projectRoot, "bin", "fabrica")
	s.Require().FileExists(s.fabricaBinary, "fabrica binary must be built")

	s.fabricaBinary, err = filepath.Abs(s.fabricaBinary)
	s.Require().NoError(err)

	s.tempDir = s.T().TempDir()
}

func (s *OTelTestSuite) TearDownTest() {
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

func (s *OTelTestSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

func (s *OTelTestSuite) TestOTelGeneration() {
	project := s.createProject("otel-generation", "github.com/test/otel-generation", "file")

	err := project.InitializeWithFlags(s.fabricaBinary, "--otel")
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.CheckGeneratedFile("internal/middleware/tracing_middleware_generated.go", "InitializeTracing")
	s.Require().NoError(err)

	err = project.CheckGeneratedFile("cmd/server/main.go", "enable-tracing")
	s.Require().NoError(err)

	err = project.CheckGeneratedFile("cmd/server/main.go", "trace-sample-ratio")
	s.Require().NoError(err)

	err = project.CheckGeneratedFile("cmd/server/main.go", "TracingMiddleware")
	s.Require().NoError(err)
}

func TestOTelTestSuite(t *testing.T) {
	suite.Run(t, new(OTelTestSuite))
}

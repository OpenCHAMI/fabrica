// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AuthMiddlewareTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

func (s *AuthMiddlewareTestSuite) SetupSuite() {
	wd, err := os.Getwd()
	s.Require().NoError(err)

	projectRoot := filepath.Join(wd, "..", "..")
	s.fabricaBinary = filepath.Join(projectRoot, "bin", "fabrica")
	s.Require().FileExists(s.fabricaBinary, "fabrica binary must be built")

	s.fabricaBinary, err = filepath.Abs(s.fabricaBinary)
	s.Require().NoError(err)

	s.tempDir = s.T().TempDir()
}

func (s *AuthMiddlewareTestSuite) TearDownTest() {
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

func (s *AuthMiddlewareTestSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

func (s *AuthMiddlewareTestSuite) TestAuthMiddlewareRoutesRegistration() {
	project := s.createProject("auth-routes-test", "github.com/test/auth-routes", "file")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	routesFile := filepath.Join(project.Dir, "cmd", "server", "routes_generated.go")
	content, err := os.ReadFile(routesFile)
	s.Require().NoError(err)

	routesContent := string(content)

	s.Assert().NotContains(routesContent, "r.Group(func(protected chi.Router)",
		"Generated routes should NOT contain nested group that shadows middleware")

	s.Assert().Contains(routesContent, "func RegisterGeneratedRoutes(r chi.Router)",
		"Generated routes should have RegisterGeneratedRoutes function")

	s.Assert().Contains(routesContent, `r.Route("/devices"`,
		"Routes should be registered directly on the parameter router")
}

func (s *AuthMiddlewareTestSuite) TestNoAuthMiddlewareBaselineWorks() {
	project := s.createProject("no-auth-test", "github.com/test/no-auth", "file")

	err := project.Initialize(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntime()
	s.Require().NoError(err)

	reqBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test-device",
		},
		"spec": map[string]interface{}{
			"description": "Test device without auth",
		},
	}
	jsonData, err := json.Marshal(reqBody)
	s.Require().NoError(err)

	resp, err := http.Post(
		fmt.Sprintf("%s/devices", project.ServerURL),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Assert().Equal(http.StatusCreated, resp.StatusCode,
		"Without auth middleware, POST should succeed (baseline behavior)")
}

func (s *AuthMiddlewareTestSuite) TestAuthMiddlewareEnforcementWithTokenSmith() {
	s.T().Skip("TODO: Implement auth enforcement test with TokenSmith mock infrastructure")
}

func TestAuthMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(AuthMiddlewareTestSuite))
}

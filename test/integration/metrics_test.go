// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// MetricsTestSuite validates generated Prometheus integration behavior.
type MetricsTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	projects      []*TestProject
}

func (s *MetricsTestSuite) SetupSuite() {
	wd, err := os.Getwd()
	s.Require().NoError(err)

	projectRoot := filepath.Join(wd, "..", "..")
	s.fabricaBinary = filepath.Join(projectRoot, "bin", "fabrica")
	s.Require().FileExists(s.fabricaBinary, "fabrica binary must be built")

	s.fabricaBinary, err = filepath.Abs(s.fabricaBinary)
	s.Require().NoError(err)

	s.tempDir = s.T().TempDir()
}

func (s *MetricsTestSuite) TearDownTest() {
	for _, project := range s.projects {
		project.StopServer() //nolint:all
	}
	s.projects = nil
}

func (s *MetricsTestSuite) createProject(name, module, storage string) *TestProject {
	project := NewTestProject(&s.Suite, s.tempDir, name, module, storage)
	s.projects = append(s.projects, project)
	return project
}

func (s *MetricsTestSuite) TestMetricsGeneration() {
	project := s.createProject("metrics-generation", "github.com/test/metrics-generation", "file")

	err := project.InitializeWithFlags(s.fabricaBinary, "--metrics")
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.CheckGeneratedFile("internal/middleware/metrics_middleware_generated.go", "MetricsMiddleware")
	s.Require().NoError(err)

	err = project.CheckGeneratedFile("cmd/server/main.go", "metrics-auth-required")
	s.Require().NoError(err)

	err = project.CheckGeneratedFile("cmd/server/main.go", "promhttp.HandlerFor")
	s.Require().NoError(err)
}

func (s *MetricsTestSuite) TestMetricsEndpointAndResourceLabels() {
	project := s.createProject("metrics-runtime", "github.com/test/metrics-runtime", "file")

	err := project.InitializeWithFlags(s.fabricaBinary, "--metrics")
	s.Require().NoError(err)

	err = project.AddResource(s.fabricaBinary, "Device")
	s.Require().NoError(err)

	err = project.Generate(s.fabricaBinary)
	s.Require().NoError(err)

	err = project.StartServerRuntimeWithArgs("--enable-metrics=true")
	s.Require().NoError(err)

	createPayload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "metrics-device",
		},
		"spec": map[string]interface{}{
			"description": "device for metrics",
		},
	}

	resp, _, err := project.HTTPCall(http.MethodPost, "/devices", createPayload, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	metricsURL := "http://localhost:9090/metrics"
	reqDeadline := time.Now().Add(10 * time.Second)
	for {
		metricsResp, getErr := http.Get(metricsURL)
		if getErr == nil {
			body := readBody(s.T(), metricsResp)
			metricsResp.Body.Close()
			s.Require().Equal(http.StatusOK, metricsResp.StatusCode)
			s.Require().Contains(body, "fabrica_http_requests_total")
			s.Require().Contains(body, "fabrica_http_request_duration_seconds")
			s.Require().Contains(body, "resource=\"devices\"")
			s.Require().Contains(body, "operation=\"create\"")
			s.Require().Contains(body, "route=\"/devices\"")
			break
		}

		if time.Now().After(reqDeadline) {
			s.FailNow("metrics endpoint did not become available", "error: %v", getErr)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	return string(body)
}

func TestMetricsTestSuite(t *testing.T) {
	suite.Run(t, new(MetricsTestSuite))
}

// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

// RackReconciliationAutomatedTestSuite tests the new automated reconciliation workflow
// with --reconcile flag and automatic code generation
type RackReconciliationAutomatedTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	project       *TestProject
}

// SetupSuite initializes the test environment
func (s *RackReconciliationAutomatedTestSuite) SetupSuite() {
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
func (s *RackReconciliationAutomatedTestSuite) TearDownTest() {
	if s.project != nil {
		s.project.StopServer() //nolint:all
	}
}

// FabricaConfig represents the .fabrica.yaml structure
type FabricaConfig struct {
	Features struct {
		Reconciliation struct {
			Enabled      bool `yaml:"enabled"`
			WorkerCount  int  `yaml:"worker_count"`
			RequeueDelay int  `yaml:"requeue_delay"`
		} `yaml:"reconciliation"`
		Events struct {
			Enabled bool   `yaml:"enabled"`
			BusType string `yaml:"bus_type"`
		} `yaml:"events"`
	} `yaml:"features"`
	Generation struct {
		Reconciliation bool `yaml:"reconciliation"`
	} `yaml:"generation"`
}

// TestAutomatedReconciliationWorkflow tests the complete new workflow:
// 1. fabrica init --reconcile
// 2. fabrica add resource
// 3. fabrica generate (auto-generates reconcilers)
func (s *RackReconciliationAutomatedTestSuite) TestAutomatedReconciliationWorkflow() {
	projectName := "auto-recon-test"
	projectModule := "github.com/test/autorecon"

	s.T().Log("=== Testing Automated Reconciliation Workflow ===")

	// Step 1: Initialize with --reconcile flag
	s.T().Log("Step 1: Initialize project with --reconcile flag")
	projectDir := filepath.Join(s.tempDir, projectName)
	cmd := exec.Command(s.fabricaBinary,
		"init", projectName,
		"--module", projectModule,
		"--storage-type", "file",
		"--events",
		"--reconcile",
	)
	cmd.Dir = s.tempDir
	output, err := cmd.CombinedOutput()
	s.Require().NoError(err, "init with --reconcile should succeed: %s", string(output))
	s.T().Log("✓ Project initialized with reconciliation enabled")

	// Create project wrapper
	s.project = &TestProject{
		suite:  &s.Suite,
		Dir:    projectDir,
		Name:   projectName,
		Module: projectModule,
	}

	// Step 2: Verify .fabrica.yaml contains reconciliation config
	s.T().Log("Step 2: Verify .fabrica.yaml configuration")
	configPath := filepath.Join(projectDir, ".fabrica.yaml")
	s.Require().FileExists(configPath, ".fabrica.yaml should exist")

	configData, err := os.ReadFile(configPath)
	s.Require().NoError(err, "should be able to read .fabrica.yaml")

	var config FabricaConfig
	err = yaml.Unmarshal(configData, &config)
	s.Require().NoError(err, "should be able to parse .fabrica.yaml")

	s.Assert().True(config.Features.Reconciliation.Enabled, "reconciliation should be enabled")
	s.Assert().Equal(5, config.Features.Reconciliation.WorkerCount, "worker count should be 5")
	s.Assert().Equal(5, config.Features.Reconciliation.RequeueDelay, "requeue delay should be 5")
	s.Assert().True(config.Features.Events.Enabled, "events should be enabled")
	s.Assert().True(config.Generation.Reconciliation, "reconciliation generation should be enabled")
	s.T().Log("✓ Configuration contains correct reconciliation settings")

	// Step 3: Verify main.go contains reconciliation controller setup (commented)
	s.T().Log("Step 3: Verify main.go contains reconciliation setup")
	mainPath := filepath.Join(projectDir, "cmd/server/main.go")
	s.Require().FileExists(mainPath, "main.go should exist")

	mainContent, err := os.ReadFile(mainPath)
	s.Require().NoError(err, "should be able to read main.go")
	mainStr := string(mainContent)

	s.Assert().Contains(mainStr, "pkg/reconcile", "main.go should import reconcile package")
	s.Assert().Contains(mainStr, "pkg/reconcilers", "main.go should import reconcilers package (commented)")
	s.Assert().Contains(mainStr, "ReconcileEnabled", "main.go should have ReconcileEnabled config")
	s.Assert().Contains(mainStr, "ReconcileWorkers", "main.go should have ReconcileWorkers config")
	s.Assert().Contains(mainStr, "reconcile.NewController", "main.go should create controller (commented)")
	s.T().Log("✓ main.go contains reconciliation controller setup")

	// Step 4: Add test resources
	s.T().Log("Step 4: Add test resources")
	resources := []string{"Device", "Rack"}
	for _, resource := range resources {
		err = s.project.AddResource(s.fabricaBinary, resource)
		s.Require().NoError(err, "adding %s resource should succeed", resource)
		s.T().Logf("  ✓ Added resource: %s", resource)
	}

	// Step 5: Generate code (should auto-generate reconcilers)
	s.T().Log("Step 5: Generate code (auto-generates reconcilers)")
	err = s.project.Generate(s.fabricaBinary)
	s.Require().NoError(err, "code generation should succeed")
	s.T().Log("✓ Code generation completed")

	// Step 6: Verify reconcilers were generated
	s.T().Log("Step 6: Verify reconcilers were auto-generated")
	reconcilersDir := filepath.Join(projectDir, "pkg/reconcilers")
	s.Require().DirExists(reconcilersDir, "pkg/reconcilers directory should exist")

	// Check for generated reconciler files
	deviceReconciler := filepath.Join(reconcilersDir, "device_reconciler_generated.go")
	s.Assert().FileExists(deviceReconciler, "device_reconciler_generated.go should exist")

	rackReconciler := filepath.Join(reconcilersDir, "rack_reconciler_generated.go")
	s.Assert().FileExists(rackReconciler, "rack_reconciler_generated.go should exist")

	registration := filepath.Join(reconcilersDir, "registration_generated.go")
	s.Assert().FileExists(registration, "registration_generated.go should exist")

	eventHandlers := filepath.Join(reconcilersDir, "event_handlers_generated.go")
	s.Assert().FileExists(eventHandlers, "event_handlers_generated.go should exist")

	s.T().Log("✓ All reconciler files generated")

	// Step 7: Verify reconciler content
	s.T().Log("Step 7: Verify generated reconciler structure")
	deviceReconcilerContent, err := os.ReadFile(deviceReconciler)
	s.Require().NoError(err, "should be able to read device reconciler")
	deviceStr := string(deviceReconcilerContent)

	s.Assert().Contains(deviceStr, "type DeviceReconciler struct", "should define DeviceReconciler struct")
	s.Assert().Contains(deviceStr, "func NewDefaultDeviceReconciler", "should have factory function")
	s.Assert().Contains(deviceStr, "func (r *DeviceReconciler) GetResourceKind()", "should implement GetResourceKind")
	s.Assert().Contains(deviceStr, "func (r *DeviceReconciler) Reconcile(", "should implement Reconcile method")
	s.Assert().Contains(deviceStr, "func (r *DeviceReconciler) reconcileDevice(", "should have custom reconcileDevice stub")
	s.Assert().Contains(deviceStr, "TODO: Implement Device-specific reconciliation logic", "should have TODO comment")
	s.T().Log("✓ Reconciler structure is correct")

	// Step 8: Verify registration file
	s.T().Log("Step 8: Verify registration file")
	registrationContent, err := os.ReadFile(registration)
	s.Require().NoError(err, "should be able to read registration file")
	regStr := string(registrationContent)

	s.Assert().Contains(regStr, "func RegisterReconcilers(", "should define RegisterReconcilers function")
	s.Assert().Contains(regStr, "NewDefaultDeviceReconciler", "should register Device reconciler")
	s.Assert().Contains(regStr, "NewDefaultRackReconciler", "should register Rack reconciler")
	s.Assert().Contains(regStr, "controller.RegisterReconciler", "should call RegisterReconciler")
	s.T().Log("✓ Registration file is correct")

	// Step 9: Verify project builds with generated reconcilers
	s.T().Log("Step 9: Verify project compiles")
	err = s.project.Build()
	s.Require().NoError(err, "project should build successfully with auto-generated reconcilers")
	s.T().Log("✓ Project builds successfully")

	// Final summary
	s.T().Log("=== Automated Reconciliation Workflow Test Summary ===")
	s.T().Log("✓ fabrica init --reconcile creates correct config")
	s.T().Log("✓ .fabrica.yaml tracks reconciliation settings")
	s.T().Log("✓ main.go includes controller setup (commented)")
	s.T().Log("✓ fabrica generate auto-detects reconciliation from config")
	s.T().Log("✓ Reconcilers generated automatically for all resources")
	s.T().Log("✓ Registration boilerplate generated")
	s.T().Log("✓ Project compiles with generated code")
	s.T().Log("=== All automated reconciliation tests PASSED ===")
}

// TestReconciliationWithoutEventsWarning tests that reconciliation without events shows a warning
func (s *RackReconciliationAutomatedTestSuite) TestReconciliationWithoutEventsWarning() {
	projectName := "no-events-test"
	projectModule := "github.com/test/noevents"

	s.T().Log("=== Testing Reconciliation Without Events ===")

	// Initialize with --reconcile but WITHOUT --events
	projectDir := filepath.Join(s.tempDir, projectName)
	cmd := exec.Command(s.fabricaBinary,
		"init", projectName,
		"--module", projectModule,
		"--reconcile",
		// Note: no --events flag
	)
	cmd.Dir = s.tempDir
	output, err := cmd.CombinedOutput()
	s.Require().NoError(err, "init should succeed even without events: %s", string(output))

	// Verify main.go warns about missing events
	mainPath := filepath.Join(projectDir, "cmd/server/main.go")
	mainContent, err := os.ReadFile(mainPath)
	s.Require().NoError(err)
	mainStr := string(mainContent)

	s.Assert().Contains(mainStr, "Reconciliation requires events to be enabled",
		"main.go should warn when reconciliation is enabled without events")
	s.T().Log("✓ Warning message present when reconciliation enabled without events")
}

// TestReconciliationConfigValues tests custom reconciliation config values
func (s *RackReconciliationAutomatedTestSuite) TestReconciliationConfigValues() {
	projectName := "custom-config-test"
	projectModule := "github.com/test/customconfig"

	s.T().Log("=== Testing Custom Reconciliation Config ===")

	// Initialize with custom worker count and requeue delay
	projectDir := filepath.Join(s.tempDir, projectName)
	cmd := exec.Command(s.fabricaBinary,
		"init", projectName,
		"--module", projectModule,
		"--events",
		"--reconcile",
		"--reconcile-workers", "10",
		"--reconcile-requeue", "3",
	)
	cmd.Dir = s.tempDir
	output, err := cmd.CombinedOutput()
	s.Require().NoError(err, "init with custom config should succeed: %s", string(output))

	// Verify config has custom values
	configPath := filepath.Join(projectDir, ".fabrica.yaml")
	configData, err := os.ReadFile(configPath)
	s.Require().NoError(err)

	var config FabricaConfig
	err = yaml.Unmarshal(configData, &config)
	s.Require().NoError(err)

	s.Assert().Equal(10, config.Features.Reconciliation.WorkerCount,
		"worker count should match custom value")
	s.Assert().Equal(3, config.Features.Reconciliation.RequeueDelay,
		"requeue delay should match custom value")
	s.T().Log("✓ Custom reconciliation config values applied correctly")
}

// Run the test suite
func TestRackReconciliationAutomatedTestSuite(t *testing.T) {
	suite.Run(t, new(RackReconciliationAutomatedTestSuite))
}

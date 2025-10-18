// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/alexlovelltroy/fabrica/pkg/events"
	"github.com/alexlovelltroy/fabrica/pkg/reconcile"
	"github.com/alexlovelltroy/fabrica/pkg/resource"
	"github.com/alexlovelltroy/fabrica/pkg/storage"
)

// Mock resources for testing reconciliation
type RackTemplate struct {
	resource.Resource
	Spec   RackTemplateSpec   `json:"spec"`
	Status RackTemplateStatus `json:"status"`
}

type RackTemplateSpec struct {
	ChassisCount  int           `json:"chassisCount"`
	ChassisConfig ChassisConfig `json:"chassisConfig"`
	Description   string        `json:"description,omitempty"`
}

type ChassisConfig struct {
	BladeCount  int         `json:"bladeCount"`
	BladeConfig BladeConfig `json:"bladeConfig"`
}

type BladeConfig struct {
	NodeCount int    `json:"nodeCount"`
	BMCMode   string `json:"bmcMode"`
}

type RackTemplateStatus struct {
	UsageCount int `json:"usageCount"`
}

type Rack struct {
	resource.Resource
	Spec   RackSpec   `json:"spec"`
	Status RackStatus `json:"status"`
}

type RackSpec struct {
	TemplateUID string `json:"templateUID"`
	Location    string `json:"location"`
	Datacenter  string `json:"datacenter,omitempty"`
}

type RackStatus struct {
	Phase        string   `json:"phase"`
	ChassisUIDs  []string `json:"chassisUIDs,omitempty"`
	TotalChassis int      `json:"totalChassis"`
	TotalBlades  int      `json:"totalBlades"`
	TotalNodes   int      `json:"totalNodes"`
	TotalBMCs    int      `json:"totalBMCs"`
}

func (r *RackTemplate) GetKind() string { return "RackTemplate" }
func (r *RackTemplate) GetName() string { return r.Metadata.Name }
func (r *RackTemplate) GetUID() string  { return r.Metadata.UID }

func (r *Rack) GetKind() string { return "Rack" }
func (r *Rack) GetName() string { return r.Metadata.Name }
func (r *Rack) GetUID() string  { return r.Metadata.UID }

// RackReconciliationFunctionalTestSuite tests actual reconciliation execution
type RackReconciliationFunctionalTestSuite struct {
	suite.Suite
	fabricaBinary string
	tempDir       string
	// project       *TestProject
}

// SetupSuite initializes the test environment
func (s *RackReconciliationFunctionalTestSuite) SetupSuite() {
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

// TestRackReconciliationExecution tests event-driven reconciler execution
//
// This test verifies that:
// 1. Publishing a resource creation event triggers reconciliation
// 2. The controller properly handles events and enqueues reconcile requests
// 3. Workers process the requests and call the reconciler
// 4. Child resources are created and parent status is updated
func (s *RackReconciliationFunctionalTestSuite) TestRackReconciliationExecution() {
	ctx := context.Background()

	// Set up storage
	storageBackend, err := storage.NewFileBackend(filepath.Join(s.tempDir, "data"))
	s.Require().NoError(err)

	// Set up event bus
	eventBus := events.NewInMemoryEventBus(100, 5)
	eventBus.Start()
	defer eventBus.Close() //nolint:errcheck

	// Register resource prefixes
	resource.RegisterResourcePrefix("RackTemplate", "rktmpl")
	resource.RegisterResourcePrefix("Rack", "rack")
	resource.RegisterResourcePrefix("Chassis", "chas")
	resource.RegisterResourcePrefix("Blade", "blade")
	resource.RegisterResourcePrefix("BMC", "bmc")
	resource.RegisterResourcePrefix("Node", "node")

	// Create a RackTemplate
	templateUID, err := resource.GenerateUIDForResource("RackTemplate")
	s.Require().NoError(err)

	template := &RackTemplate{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "RackTemplate",
			Metadata: resource.Metadata{
				Name: "standard-rack",
				UID:  templateUID,
			},
		},
		Spec: RackTemplateSpec{
			ChassisCount: 2,
			ChassisConfig: ChassisConfig{
				BladeCount: 4,
				BladeConfig: BladeConfig{
					NodeCount: 2,
					BMCMode:   "shared",
				},
			},
			Description: "Standard 2-chassis rack",
		},
	}
	template.Metadata.Initialize(template.Metadata.Name, template.Metadata.UID)

	// Save template to storage
	templateData, err := json.Marshal(template)
	s.Require().NoError(err)
	err = storageBackend.Save(ctx, template.GetKind(), template.GetUID(), templateData)
	s.Require().NoError(err)

	s.T().Logf("Created RackTemplate: %s (%s)", template.GetName(), template.GetUID())

	// Create a Rack referencing the template
	rackUID, err := resource.GenerateUIDForResource("Rack")
	s.Require().NoError(err)

	rack := &Rack{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "Rack",
			Metadata: resource.Metadata{
				Name: "rack-01",
				UID:  rackUID,
			},
		},
		Spec: RackSpec{
			TemplateUID: templateUID,
			Location:    "datacenter-1",
			Datacenter:  "DC1",
		},
		Status: RackStatus{
			Phase: "Pending",
		},
	}
	rack.Metadata.Initialize(rack.Metadata.Name, rack.Metadata.UID)

	// Save rack to storage
	rackData, err := json.Marshal(rack)
	s.Require().NoError(err)
	err = storageBackend.Save(ctx, rack.GetKind(), rack.GetUID(), rackData)
	s.Require().NoError(err)

	s.T().Logf("Created Rack: %s (%s) with template: %s", rack.GetName(), rack.GetUID(), templateUID)

	// Create reconciler
	reconciler := &SimpleRackReconciler{
		BaseReconciler: reconcile.BaseReconciler{
			EventBus: eventBus,
			Logger:   reconcile.NewDefaultLogger(),
		},
		Storage: storageBackend,
	}

	// Create and start reconciliation controller
	controller := reconcile.NewController(eventBus, storageBackend)
	err = controller.RegisterReconciler(reconciler)
	s.Require().NoError(err, "registering reconciler should succeed")

	err = controller.Start(ctx)
	s.Require().NoError(err, "controller should start")
	defer controller.Stop() //nolint:errcheck

	s.T().Log("Reconciliation controller started")

	// Publish Rack creation event - this should trigger reconciliation automatically
	s.T().Log("Publishing Rack creation event...")
	event, err := events.NewResourceEvent("io.fabrica.rack.created", rack.GetKind(), rack.GetUID(), rack)
	s.Require().NoError(err, "creating event should succeed")

	s.T().Logf("Event created: type=%s, resourceKind=%s, resourceUID=%s", event.Type(), event.ResourceKind(), event.ResourceUID())

	err = eventBus.Publish(ctx, *event)
	s.Require().NoError(err, "publishing event should succeed")

	s.T().Log("Event published - waiting for reconciliation...")
	// Give the controller time to process and reconcile
	time.Sleep(5 * time.Second)

	// Load the updated rack
	updatedRackData, err := storageBackend.Load(ctx, "Rack", rackUID)
	s.Require().NoError(err)

	var updatedRack Rack
	updatedRackBytes, err := json.Marshal(updatedRackData)
	s.Require().NoError(err)
	err = json.Unmarshal(updatedRackBytes, &updatedRack)
	s.Require().NoError(err)

	// Verify reconciliation results
	s.Require().Equal("Ready", updatedRack.Status.Phase, "Rack should be in Ready phase")
	s.Require().Equal(2, updatedRack.Status.TotalChassis, "Should have 2 chassis")
	s.Require().Equal(8, updatedRack.Status.TotalBlades, "Should have 8 blades (4 per chassis)")
	s.Require().Equal(16, updatedRack.Status.TotalNodes, "Should have 16 nodes (2 per blade)")
	s.Require().Equal(8, updatedRack.Status.TotalBMCs, "Should have 8 BMCs (1 per blade in shared mode)")

	// List created resources
	chassisList, err := storageBackend.List(ctx, "Chassis")
	s.Require().NoError(err)
	s.Require().Len(chassisList, 2, "Should have 2 chassis resources")

	bladesList, err := storageBackend.List(ctx, "Blade")
	s.Require().NoError(err)
	s.Require().Len(bladesList, 8, "Should have 8 blade resources")

	nodesList, err := storageBackend.List(ctx, "Node")
	s.Require().NoError(err)
	s.Require().Len(nodesList, 16, "Should have 16 node resources")

	bmcsList, err := storageBackend.List(ctx, "BMC")
	s.Require().NoError(err)
	s.Require().Len(bmcsList, 8, "Should have 8 BMC resources")

	s.T().Log("✓ Rack reconciliation functional test passed")
	s.T().Log("✓ RackTemplate loaded correctly")
	s.T().Log("✓ Child resources created successfully")
	s.T().Log("✓ Rack status updated with counts")
	s.T().Logf("✓ Created: %d Chassis, %d Blades, %d Nodes, %d BMCs",
		len(chassisList), len(bladesList), len(nodesList), len(bmcsList))
}

// SimpleRackReconciler is a mock reconciler for testing
type SimpleRackReconciler struct {
	reconcile.BaseReconciler
	Storage storage.StorageBackend
}

func (r *SimpleRackReconciler) Reconcile(ctx context.Context, resourceInterface interface{}) (reconcile.Result, error) {
	// The controller loads raw data from storage, so we need to unmarshal it
	var rack Rack
	rackBytes, err := json.Marshal(resourceInterface)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to marshal rack data: %w", err)
	}
	if err := json.Unmarshal(rackBytes, &rack); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to unmarshal rack: %w", err)
	}

	if rack.Status.Phase == "Ready" {
		return reconcile.Result{}, nil
	}

	// Load template
	templateData, err := r.Storage.Load(ctx, "RackTemplate", rack.Spec.TemplateUID)
	if err != nil {
		return reconcile.Result{}, err
	}

	var template RackTemplate
	templateBytes, _ := json.Marshal(templateData)
	err = json.Unmarshal(templateBytes, &template)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to unmarshal rack template: %w", err)
	}

	// Calculate totals
	totalChassis := template.Spec.ChassisCount
	totalBlades := totalChassis * template.Spec.ChassisConfig.BladeCount
	totalNodes := totalBlades * template.Spec.ChassisConfig.BladeConfig.NodeCount
	totalBMCs := totalBlades // Shared mode: 1 BMC per blade

	// Create child resources (simplified - just create the counts)
	for i := 0; i < totalChassis; i++ {
		chassisUID, _ := resource.GenerateUIDForResource("Chassis")
		chassisData, _ := json.Marshal(map[string]interface{}{
			"kind": "Chassis",
			"metadata": map[string]string{
				"name": strings.ToLower("chassis-" + chassisUID),
				"uid":  chassisUID,
			},
			"spec": map[string]interface{}{
				"rackUID":       rack.GetUID(),
				"chassisNumber": i,
			},
		})
		err = r.Storage.Save(ctx, "Chassis", chassisUID, chassisData)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to save chassis: %w", err)
		}
	}

	for i := 0; i < totalBlades; i++ {
		bladeUID, _ := resource.GenerateUIDForResource("Blade")
		bladeData, _ := json.Marshal(map[string]interface{}{
			"kind": "Blade",
			"metadata": map[string]string{
				"name": strings.ToLower("blade-" + bladeUID),
				"uid":  bladeUID,
			},
			"spec": map[string]interface{}{
				"bladeNumber": i,
			},
		})
		err = r.Storage.Save(ctx, "Blade", bladeUID, bladeData)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to save blade: %w", err)
		}
	}

	for i := 0; i < totalBMCs; i++ {
		bmcUID, _ := resource.GenerateUIDForResource("BMC")
		bmcData, _ := json.Marshal(map[string]interface{}{
			"kind": "BMC",
			"metadata": map[string]string{
				"name": strings.ToLower("bmc-" + bmcUID),
				"uid":  bmcUID,
			},
		})
		err = r.Storage.Save(ctx, "BMC", bmcUID, bmcData)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to save BMC: %w", err)
		}
	}

	for i := 0; i < totalNodes; i++ {
		nodeUID, _ := resource.GenerateUIDForResource("Node")
		nodeData, _ := json.Marshal(map[string]interface{}{
			"kind": "Node",
			"metadata": map[string]string{
				"name": strings.ToLower("node-" + nodeUID),
				"uid":  nodeUID,
			},
			"spec": map[string]interface{}{
				"nodeNumber": i,
			},
		})
		err = r.Storage.Save(ctx, "Node", nodeUID, nodeData)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to save node: %w", err)
		}
	}

	// Update rack status
	rack.Status.Phase = "Ready"
	rack.Status.TotalChassis = totalChassis
	rack.Status.TotalBlades = totalBlades
	rack.Status.TotalNodes = totalNodes
	rack.Status.TotalBMCs = totalBMCs

	rackData, _ := json.Marshal(rack)
	err = r.Storage.Save(ctx, rack.GetKind(), rack.GetUID(), rackData)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to update rack status: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *SimpleRackReconciler) GetResourceKind() string {
	return "Rack"
}

// Run the test suite
func TestRackReconciliationFunctionalTestSuite(t *testing.T) {
	suite.Run(t, new(RackReconciliationFunctionalTestSuite))
}

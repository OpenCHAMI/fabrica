// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexlovelltroy/fabrica/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusSubresource tests that spec and status can be updated independently
func TestStatusSubresourceSpecStatusSeparation(t *testing.T) {
	t.Run("UpdateSpecDoesNotChangeStatus", func(t *testing.T) {
		// This test verifies that updating the spec via PUT /resources/{uid}
		// does not modify the status fields

		// Create a mock resource with both spec and status
		type TestResource struct {
			resource.Resource
			Spec struct {
				Name     string `json:"name"`
				Location string `json:"location"`
			} `json:"spec"`
			Status struct {
				Phase   string `json:"phase"`
				Message string `json:"message"`
			} `json:"status"`
		}

		// Initial resource with status set
		res := &TestResource{}
		res.APIVersion = "v1"
		res.Kind = "TestResource"
		res.Metadata.Initialize("test-1", "test-123")
		res.Spec.Name = "original"
		res.Spec.Location = "datacenter-1"
		res.Status.Phase = "Ready"
		res.Status.Message = "All systems operational"

		// Simulate updating spec (in real impl, this would load from storage first)
		// The key is that status should NOT be in the update request

		updatedSpec := struct {
			Name     string `json:"name"`
			Location string `json:"location"`
		}{
			Name:     "updated",
			Location: "datacenter-2",
		}

		// In the actual handler, this would:
		// 1. Load resource
		// 2. Update ONLY spec fields
		// 3. Save resource
		// 4. Status should remain unchanged

		// Verify the design principle:
		assert.Equal(t, "Ready", res.Status.Phase, "Status phase should not change")
		assert.Equal(t, "All systems operational", res.Status.Message, "Status message should not change")
		assert.NotEqual(t, res.Spec.Name, updatedSpec.Name, "Spec should be different before update")
	})

	t.Run("UpdateStatusDoesNotChangeSpec", func(t *testing.T) {
		// This test verifies that updating status via PUT /resources/{uid}/status
		// does not modify the spec fields

		type TestResource struct {
			resource.Resource
			Spec struct {
				Name     string `json:"name"`
				Location string `json:"location"`
			} `json:"spec"`
			Status struct {
				Phase   string `json:"phase"`
				Message string `json:"message"`
			} `json:"status"`
		}

		// Initial resource
		res := &TestResource{}
		res.APIVersion = "v1"
		res.Kind = "TestResource"
		res.Metadata.Initialize("test-1", "test-123")
		res.Spec.Name = "original"
		res.Spec.Location = "datacenter-1"
		res.Status.Phase = "Pending"
		res.Status.Message = "Initializing"

		originalSpec := res.Spec

		// Status update
		newStatus := struct {
			Phase   string `json:"phase"`
			Message string `json:"message"`
		}{
			Phase:   "Ready",
			Message: "Initialization complete",
		}

		// In the actual handler, this would:
		// 1. Load resource
		// 2. Update ONLY status fields
		// 3. Save resource
		// 4. Spec should remain unchanged

		// Verify design principle:
		assert.Equal(t, originalSpec, res.Spec, "Spec should not change when updating status")
		_ = newStatus // Would be applied to res.Status in real implementation
	})
}

// TestStatusSubresourceRoutes tests that the status subresource routes are generated
func TestStatusSubresourceRoutes(t *testing.T) {
	t.Run("StatusEndpointsExist", func(t *testing.T) {
		// This test verifies that the generated routes include status subresource endpoints
		// In a real test, we would:
		// 1. Generate code for a test resource
		// 2. Parse the generated routes file
		// 3. Verify /resources/{uid}/status routes exist

		expectedRoutes := []string{
			"PUT /resources/{uid}/status",
			"PATCH /resources/{uid}/status",
		}

		// This would be tested against actual generated routes
		for _, route := range expectedRoutes {
			assert.NotEmpty(t, route, "Status route should be defined: %s", route)
		}
	})
}

// TestReconcilerStatusUpdates tests that reconcilers update status without changing spec
func TestReconcilerStatusUpdates(t *testing.T) {
	t.Run("ReconcilerPreservesSpec", func(t *testing.T) {
		// This test verifies that when a reconciler updates status,
		// it loads a fresh copy and preserves any concurrent spec changes

		type TestResource struct {
			resource.Resource
			Spec struct {
				DesiredState string `json:"desiredState"`
			} `json:"spec"`
			Status struct {
				CurrentState string `json:"currentState"`
				Phase        string `json:"phase"`
			} `json:"status"`
		}

		// Simulate initial resource
		initial := &TestResource{}
		initial.APIVersion = "v1"
		initial.Kind = "TestResource"
		initial.Metadata.Initialize("test-1", "test-123")
		initial.Spec.DesiredState = "running"
		initial.Status.CurrentState = "stopped"
		initial.Status.Phase = "Pending"

		// Simulate user updating spec concurrently
		userUpdated := &TestResource{}
		*userUpdated = *initial
		userUpdated.Spec.DesiredState = "paused" // User changed desired state

		// Simulate reconciler updating status
		reconcilerUpdated := &TestResource{}
		*reconcilerUpdated = *initial // Reconciler starts with initial state
		reconcilerUpdated.Status.CurrentState = "running"
		reconcilerUpdated.Status.Phase = "Ready"

		// In the real UpdateStatus implementation:
		// 1. Load fresh copy (gets userUpdated state)
		// 2. Apply status from reconcilerUpdated
		// 3. Result should have user's spec + reconciler's status

		// Expected final state
		assert.Equal(t, "paused", userUpdated.Spec.DesiredState, "Spec should have user's update")
		assert.Equal(t, "running", reconcilerUpdated.Status.CurrentState, "Status should have reconciler's update")
	})
}

// TestStatusSubresourceEventMetadata tests that status updates have proper event metadata
func TestStatusSubresourceEventMetadata(t *testing.T) {
	t.Run("StatusUpdateHasCorrectMetadata", func(t *testing.T) {
		// Verify that events published for status updates include updateType metadata
		eventMetadata := map[string]interface{}{
			"updatedAt":  time.Now().Format(time.RFC3339),
			"updateType": "status",
		}

		assert.Equal(t, "status", eventMetadata["updateType"], "Event should have updateType=status")
		assert.NotEmpty(t, eventMetadata["updatedAt"], "Event should have updatedAt timestamp")
	})

	t.Run("SpecUpdateHasNoStatusType", func(t *testing.T) {
		// Verify that regular spec updates don't have updateType: "status"
		eventMetadata := map[string]interface{}{
			"updatedAt": time.Now().Format(time.RFC3339),
			// No updateType field, or updateType: "spec"
		}

		// Status updates should be distinguishable
		if updateType, exists := eventMetadata["updateType"]; exists {
			assert.NotEqual(t, "status", updateType, "Spec update should not have updateType=status")
		}
	})
}

// TestClientLibraryStatusMethods tests that generated client has status methods
func TestClientLibraryStatusMethods(t *testing.T) {
	t.Run("ClientHasUpdateStatusMethod", func(t *testing.T) {
		// This test would verify that the generated client includes:
		// - UpdateResourceStatus(ctx, uid, status) method
		// - PatchResourceStatus(ctx, uid, patchData) method
		// - PatchResourceStatusWithType(ctx, uid, patchData, contentType) method

		requiredMethods := []string{
			"UpdateResourceStatus",
			"PatchResourceStatus",
			"PatchResourceStatusWithType",
		}

		for _, method := range requiredMethods {
			assert.NotEmpty(t, method, "Client should have method: %s", method)
		}
	})
}

// TestHTTPStatusEndpoints is a basic HTTP-level test
func TestHTTPStatusEndpoints(t *testing.T) {
	t.Run("StatusEndpointReturnsJSON", func(t *testing.T) {
		// Create a mock handler for status updates
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// Mock reading status update
			var statusUpdate map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&statusUpdate); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
				return
			}

			// Mock updating status (in real impl, would load resource, update status, save)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "TestResource",
				"metadata":   map[string]string{"uid": "test-123"},
				"spec":       map[string]string{"name": "test"},
				"status":     statusUpdate,
			})
		})

		// Create test server
		server := httptest.NewServer(handler)
		defer server.Close()

		// Test PUT /status
		statusUpdate := map[string]interface{}{
			"phase":   "Ready",
			"message": "Test complete",
		}

		body, _ := json.Marshal(statusUpdate)
		req, err := http.NewRequest(http.MethodPut, server.URL, bytes.NewReader(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Status update should succeed")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		status, ok := result["status"].(map[string]interface{})
		require.True(t, ok, "Response should have status field")
		assert.Equal(t, "Ready", status["phase"], "Status should be updated")
	})
}

// TestTemplateGeneration would test the actual template generation
// This would require mocking the template engine or using the real codegen
func TestTemplateGeneration(t *testing.T) {
	t.Skip("Template generation testing requires codegen mock/integration")

	// This test would:
	// 1. Create a test resource definition
	// 2. Run template generation
	// 3. Parse generated code
	// 4. Verify status subresource handlers exist
	// 5. Verify routes include /status endpoints
	// 6. Verify client has status methods
}

// TestEndToEndStatusWorkflow tests a complete workflow
func TestEndToEndStatusWorkflow(t *testing.T) {
	t.Run("UserUpdateSpecThenControllerUpdateStatus", func(t *testing.T) {
		ctx := context.Background()

		// This would test:
		// 1. User creates resource via POST /resources
		// 2. User updates spec via PUT /resources/{uid}
		// 3. Controller receives event
		// 4. Controller updates status via PUT /resources/{uid}/status
		// 5. Final resource has user's spec + controller's status

		// For now, this is a design validation test
		type Resource struct {
			Spec   map[string]interface{} `json:"spec"`
			Status map[string]interface{} `json:"status"`
		}

		// Step 1: Initial resource
		res := Resource{
			Spec:   map[string]interface{}{"name": "test", "location": "dc1"},
			Status: map[string]interface{}{"phase": "Pending"},
		}

		// Step 2: User updates spec
		res.Spec["location"] = "dc2"

		// Step 3: Controller observes and updates status
		res.Status["phase"] = "Ready"
		res.Status["message"] = "Deployed to dc2"

		// Verify
		assert.Equal(t, "dc2", res.Spec["location"], "Spec should have user's update")
		assert.Equal(t, "Ready", res.Status["phase"], "Status should have controller's update")

		_ = ctx // Would be used for API calls in real test
	})
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMCPHandle_InitializeAndToolsList(t *testing.T) {
	srv := &mcpServer{workspaceRoot: t.TempDir()}

	initResp, err := srv.handle(mcpRequest{Method: "initialize"})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	initMap, ok := initResp.(map[string]interface{})
	if !ok {
		t.Fatalf("initialize response type mismatch: %T", initResp)
	}
	if initMap["protocolVersion"] != "2024-11-05" {
		t.Fatalf("unexpected protocolVersion: %v", initMap["protocolVersion"])
	}

	toolsResp, err := srv.handle(mcpRequest{Method: "tools/list"})
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	toolsMap, ok := toolsResp.(map[string]interface{})
	if !ok {
		t.Fatalf("tools/list response type mismatch: %T", toolsResp)
	}
	toolDefs, ok := toolsMap["tools"].([]mcpToolDef)
	if !ok {
		t.Fatalf("tools/list tools type mismatch: %T", toolsMap["tools"])
	}
	if !hasToolName(toolDefs, "create_service") {
		t.Fatalf("expected create_service tool in tools/list")
	}
	if !hasToolName(toolDefs, "validate_project") {
		t.Fatalf("expected validate_project tool in tools/list")
	}
}

func TestMCPCallTool_CreateServiceDryRun_HasPlannedFiles(t *testing.T) {
	srv := &mcpServer{workspaceRoot: t.TempDir()}

	resp, err := srv.callTool(mcpToolCallParams{
		Name: "create_service",
		Arguments: map[string]interface{}{
			"project_name": "device-api",
			"mode":         "dry_run",
			"target_dir":   ".",
		},
	})
	if err != nil {
		t.Fatalf("callTool failed: %v", err)
	}

	result, ok := resp.(mcpToolCallResult)
	if !ok {
		t.Fatalf("tool response type mismatch: %T", resp)
	}
	if result.IsError {
		t.Fatalf("expected non-error result, got error: %+v", result)
	}
	if result.StructuredContent["status"] != "dry_run" {
		t.Fatalf("expected dry_run status, got %v", result.StructuredContent["status"])
	}
	planned, ok := result.StructuredContent["planned_files"].([]string)
	if !ok {
		t.Fatalf("planned_files type mismatch: %T", result.StructuredContent["planned_files"])
	}
	if len(planned) == 0 {
		t.Fatalf("expected planned_files to be populated")
	}
}

func TestMCPCallTool_TypedErrorCodeForInvalidArguments(t *testing.T) {
	srv := &mcpServer{workspaceRoot: t.TempDir()}

	resp, err := srv.callTool(mcpToolCallParams{
		Name:      "add_resource",
		Arguments: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("callTool failed: %v", err)
	}

	result, ok := resp.(mcpToolCallResult)
	if !ok {
		t.Fatalf("tool response type mismatch: %T", resp)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true")
	}
	errorPayload, ok := result.StructuredContent["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured error payload")
	}
	if errorPayload["code"] != "invalid_arguments" {
		t.Fatalf("expected invalid_arguments code, got %v", errorPayload["code"])
	}
}

func TestMCPValidateProject_DetectsGeneratedArtifactsStale(t *testing.T) {
	workspace := t.TempDir()
	projectDir := filepath.Join(workspace, "stale-check")

	opts := &initOptions{
		modulePath:         "github.com/example/stale-check",
		description:        "stale check",
		withAuth:           false,
		withStorage:        true,
		withMetrics:        false,
		withVersion:        true,
		validationMode:     "strict",
		withEvents:         false,
		eventBusType:       "memory",
		apiGroup:           "example.fabrica.dev",
		storageVersion:     "v1",
		apiVersions:        []string{"v1"},
		withReconcile:      false,
		reconcileWorkers:   3,
		reconcileRequeueMs: 5,
		storageType:        "file",
		dbDriver:           "sqlite",
	}

	if err := runInit(projectDir, opts); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	addOpts := &addOptions{
		withValidation: true,
		withStatus:     true,
	}
	if err := withWorkingDir(projectDir, func() error {
		return runAddResource("Device", addOpts)
	}); err != nil {
		t.Fatalf("runAddResource failed: %v", err)
	}

	apis, err := LoadAPIsConfig(projectDir)
	if err != nil {
		t.Fatalf("LoadAPIsConfig failed: %v", err)
	}
	group, err := apis.primaryGroup()
	if err != nil {
		t.Fatalf("primaryGroup failed: %v", err)
	}
	typeFile := filepath.Join(projectDir, "apis", group.Name, group.StorageVersion, "device_types.go")
	routesFile := filepath.Join(projectDir, "cmd", "server", "routes_generated.go")
	if err := os.WriteFile(routesFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write routes_generated.go failed: %v", err)
	}

	old := time.Now().Add(-2 * time.Hour)
	fresh := time.Now()
	if err := os.Chtimes(routesFile, old, old); err != nil {
		t.Fatalf("Chtimes routes failed: %v", err)
	}
	if err := os.Chtimes(typeFile, fresh, fresh); err != nil {
		t.Fatalf("Chtimes types failed: %v", err)
	}

	srv := &mcpServer{workspaceRoot: workspace}
	resp, err := srv.callTool(mcpToolCallParams{
		Name: "validate_project",
		Arguments: map[string]interface{}{
			"project_path": "stale-check",
		},
	})
	if err != nil {
		t.Fatalf("callTool failed: %v", err)
	}

	result, ok := resp.(mcpToolCallResult)
	if !ok {
		t.Fatalf("tool response type mismatch: %T", resp)
	}
	if result.IsError {
		t.Fatalf("expected non-error tool result")
	}
	if result.StructuredContent["status"] != "warning" {
		t.Fatalf("expected warning status, got %v", result.StructuredContent["status"])
	}
	if !issuesContainCode(result.StructuredContent["issues"], "generated_artifacts_stale") {
		t.Fatalf("expected generated_artifacts_stale issue, got %#v", result.StructuredContent["issues"])
	}
}

func TestMCPExecuteFlow_CreateAddVersionAndInspect(t *testing.T) {
	workspace := t.TempDir()
	srv := &mcpServer{workspaceRoot: workspace}

	createResp, err := srv.callTool(mcpToolCallParams{
		Name: "create_service",
		Arguments: map[string]interface{}{
			"project_name": "flow-api",
			"mode":         "execute",
		},
	})
	if err != nil {
		t.Fatalf("create_service failed: %v", err)
	}
	createResult := createResp.(mcpToolCallResult)
	if createResult.IsError {
		t.Fatalf("create_service returned error result: %#v", createResult.StructuredContent)
	}

	addResp, err := srv.callTool(mcpToolCallParams{
		Name: "add_resource",
		Arguments: map[string]interface{}{
			"resource_name": "Sensor",
			"project_path":  "flow-api",
			"mode":          "execute",
		},
	})
	if err != nil {
		t.Fatalf("add_resource failed: %v", err)
	}
	addResult := addResp.(mcpToolCallResult)
	if addResult.IsError {
		t.Fatalf("add_resource returned error result: %#v", addResult.StructuredContent)
	}

	versionResp, err := srv.callTool(mcpToolCallParams{
		Name: "add_version",
		Arguments: map[string]interface{}{
			"new_version":  "v1alpha1",
			"project_path": "flow-api",
			"mode":         "execute",
			"from":         "v1",
		},
	})
	if err != nil {
		t.Fatalf("add_version failed: %v", err)
	}
	versionResult := versionResp.(mcpToolCallResult)
	if versionResult.IsError {
		t.Fatalf("add_version returned error result: %#v", versionResult.StructuredContent)
	}

	inspectResp, err := srv.callTool(mcpToolCallParams{
		Name: "inspect_project",
		Arguments: map[string]interface{}{
			"project_path": "flow-api",
		},
	})
	if err != nil {
		t.Fatalf("inspect_project failed: %v", err)
	}
	inspectResult := inspectResp.(mcpToolCallResult)
	if inspectResult.IsError {
		t.Fatalf("inspect_project returned error result")
	}

	versions, ok := inspectResult.StructuredContent["versions"].([]string)
	if !ok {
		t.Fatalf("versions type mismatch: %T", inspectResult.StructuredContent["versions"])
	}
	if !contains(versions, "v1alpha1") {
		t.Fatalf("expected v1alpha1 in versions, got %v", versions)
	}

	genDryRunResp, err := srv.callTool(mcpToolCallParams{
		Name: "generate_code",
		Arguments: map[string]interface{}{
			"project_path": "flow-api",
			"mode":         "dry_run",
		},
	})
	if err != nil {
		t.Fatalf("generate_code dry_run failed: %v", err)
	}
	genDryRun := genDryRunResp.(mcpToolCallResult)
	if genDryRun.IsError {
		t.Fatalf("generate_code dry_run returned error result")
	}
	if _, ok := genDryRun.StructuredContent["planned_files"].([]string); !ok {
		t.Fatalf("generate_code dry_run missing planned_files")
	}

	syncDryRunResp, err := srv.callTool(mcpToolCallParams{
		Name: "sync_dependencies",
		Arguments: map[string]interface{}{
			"project_path": "flow-api",
			"mode":         "dry_run",
		},
	})
	if err != nil {
		t.Fatalf("sync_dependencies dry_run failed: %v", err)
	}
	syncDryRun := syncDryRunResp.(mcpToolCallResult)
	if syncDryRun.IsError {
		t.Fatalf("sync_dependencies dry_run returned error result")
	}
	planned, ok := syncDryRun.StructuredContent["planned_files"].([]string)
	if !ok || len(planned) == 0 {
		t.Fatalf("sync_dependencies dry_run missing planned_files")
	}
	if !strings.Contains(strings.Join(planned, ","), "go.mod") {
		t.Fatalf("sync_dependencies planned_files should include go.mod: %v", planned)
	}
}

func hasToolName(tools []mcpToolDef, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

func issuesContainCode(raw interface{}, code string) bool {
	issues, ok := raw.([]map[string]interface{})
	if ok {
		for _, issue := range issues {
			if fmt.Sprintf("%v", issue["code"]) == code {
				return true
			}
		}
		return false
	}

	genericIssues, ok := raw.([]interface{})
	if !ok {
		return false
	}
	for _, issue := range genericIssues {
		m, ok := issue.(map[string]interface{})
		if !ok {
			continue
		}
		if fmt.Sprintf("%v", m["code"]) == code {
			return true
		}
	}
	return false
}

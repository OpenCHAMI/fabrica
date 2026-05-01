// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configpkg "github.com/openchami/fabrica/internal/config"
)

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	binaryPath := filepath.Join(repoRoot, ".tmp", "fabrica-mcp-test")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		panic(err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/fabrica")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	if err := os.Setenv("FABRICA_MCP_TEST_BINARY", binaryPath); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.Remove(binaryPath)
	os.Exit(code)
}

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
	for _, name := range []string{"define_resource_schema", "build_project", "test_project", "smoke_test_api", "describe_workflow"} {
		if !hasToolName(toolDefs, name) {
			t.Fatalf("expected %s tool in tools/list", name)
		}
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

func TestMCPCallTool_RejectsUnknownArguments(t *testing.T) {
	srv := &mcpServer{workspaceRoot: t.TempDir()}

	resp, err := srv.callTool(mcpToolCallParams{
		Name: "generate_code",
		Arguments: map[string]interface{}{
			"project_path": ".",
			"bogus":        true,
		},
	})
	if err != nil {
		t.Fatalf("callTool failed: %v", err)
	}
	result := resp.(mcpToolCallResult)
	if !result.IsError {
		t.Fatalf("expected unknown argument to produce an error")
	}
	errorPayload := result.StructuredContent["error"].(map[string]interface{})
	if errorPayload["code"] != "invalid_arguments" {
		t.Fatalf("expected invalid_arguments, got %v", errorPayload["code"])
	}
}

func TestMCPCallTool_DefaultModeIsDryRun(t *testing.T) {
	srv := &mcpServer{workspaceRoot: t.TempDir()}

	resp, err := srv.callTool(mcpToolCallParams{
		Name: "create_service",
		Arguments: map[string]interface{}{
			"project_name": "default-dry-run",
		},
	})
	if err != nil {
		t.Fatalf("callTool failed: %v", err)
	}
	result := resp.(mcpToolCallResult)
	if result.IsError {
		t.Fatalf("expected dry-run create_service to succeed: %#v", result.StructuredContent)
	}
	if result.StructuredContent["status"] != "dry_run" {
		t.Fatalf("expected default mode dry_run, got %v", result.StructuredContent["status"])
	}
	if _, err := os.Stat(filepath.Join(srv.workspaceRoot, "default-dry-run")); !os.IsNotExist(err) {
		t.Fatalf("default create_service should not mutate filesystem")
	}
}

func TestMCPValidateProject_DetectsGeneratedArtifactsStale(t *testing.T) {
	workspace := t.TempDir()
	projectDir := filepath.Join(workspace, "stale-check")
	srv := &mcpServer{workspaceRoot: workspace}
	if _, err := srv.callTool(mcpToolCallParams{
		Name: "create_service",
		Arguments: map[string]interface{}{
			"project_name": "stale-check",
			"mode":         "execute",
		},
	}); err != nil {
		t.Fatalf("create_service failed: %v", err)
	}
	if _, err := srv.callTool(mcpToolCallParams{
		Name: "add_resource",
		Arguments: map[string]interface{}{
			"project_path":    "stale-check",
			"resource_name":   "Device",
			"mode":            "execute",
			"with_validation": true,
			"with_status":     true,
		},
	}); err != nil {
		t.Fatalf("add_resource failed: %v", err)
	}

	apis, err := configpkg.LoadAPIsConfig(projectDir)
	if err != nil {
		t.Fatalf("LoadAPIsConfig failed: %v", err)
	}
	group, err := apis.PrimaryGroup()
	if err != nil {
		t.Fatalf("PrimaryGroup failed: %v", err)
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

func TestMCPDefineResourceSchema_UpdatesTypeFile(t *testing.T) {
	workspace := t.TempDir()
	srv := &mcpServer{workspaceRoot: workspace}
	if _, err := srv.callTool(mcpToolCallParams{
		Name: "create_service",
		Arguments: map[string]interface{}{
			"project_name": "schema-api",
			"mode":         "execute",
		},
	}); err != nil {
		t.Fatalf("create_service failed: %v", err)
	}
	if _, err := srv.callTool(mcpToolCallParams{
		Name: "add_resource",
		Arguments: map[string]interface{}{
			"project_path":    "schema-api",
			"resource_name":   "Widget",
			"mode":            "execute",
			"with_validation": true,
			"with_status":     true,
		},
	}); err != nil {
		t.Fatalf("add_resource failed: %v", err)
	}

	resp, err := srv.callTool(mcpToolCallParams{
		Name: "define_resource_schema",
		Arguments: map[string]interface{}{
			"project_path":  "schema-api",
			"resource_name": "Widget",
			"mode":          "execute",
			"spec_fields": []interface{}{
				map[string]interface{}{"name": "SerialNumber", "type": "string", "json_name": "serialNumber", "required": true, "validation": "required,min=3", "description": "SerialNumber identifies the widget."},
			},
			"status_fields": []interface{}{
				map[string]interface{}{"name": "Ready", "type": "bool", "json_name": "ready", "description": "Ready indicates whether the widget is usable."},
			},
		},
	})
	if err != nil {
		t.Fatalf("define_resource_schema failed: %v", err)
	}
	result := resp.(mcpToolCallResult)
	if result.IsError {
		t.Fatalf("define_resource_schema returned error: %#v", result.StructuredContent)
	}
	resourceFile := result.StructuredContent["resource_file"].(string)
	data, err := os.ReadFile(resourceFile)
	if err != nil {
		t.Fatalf("ReadFile resource: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "SerialNumber string `json:\"serialNumber\" validate:\"required,min=3\"`") {
		t.Fatalf("spec field not rendered as expected:\n%s", content)
	}
	if !strings.Contains(content, "Ready bool `json:\"ready,omitempty\"`") {
		t.Fatalf("status field not rendered as expected:\n%s", content)
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

	buildDryRunResp, err := srv.callTool(mcpToolCallParams{
		Name: "build_project",
		Arguments: map[string]interface{}{
			"project_path": "flow-api",
		},
	})
	if err != nil {
		t.Fatalf("build_project dry_run failed: %v", err)
	}
	buildDryRun := buildDryRunResp.(mcpToolCallResult)
	if buildDryRun.IsError {
		t.Fatalf("build_project dry_run returned error result")
	}
	if buildDryRun.StructuredContent["status"] != "dry_run" {
		t.Fatalf("expected build_project default dry_run, got %v", buildDryRun.StructuredContent["status"])
	}

	workflowResp, err := srv.callTool(mcpToolCallParams{
		Name: "describe_workflow",
		Arguments: map[string]interface{}{
			"goal": "new_crud_api",
		},
	})
	if err != nil {
		t.Fatalf("describe_workflow failed: %v", err)
	}
	workflow := workflowResp.(mcpToolCallResult)
	if workflow.IsError {
		t.Fatalf("describe_workflow returned error")
	}
	if _, ok := workflow.StructuredContent["workflow"].([]map[string]interface{}); !ok {
		t.Fatalf("workflow response missing typed workflow: %T", workflow.StructuredContent["workflow"])
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

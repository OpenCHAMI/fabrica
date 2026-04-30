// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const latestNegotiatedProtocolVersion = "2025-11-25"

func TestMCPToolsList_HasExpectedTools(t *testing.T) {
	srv := &mcpServer{workspaceRoot: t.TempDir()}
	toolDefs := srv.tools()
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

func TestMCPAutoReader_ContentLengthInput(t *testing.T) {
	mode := &mcpWireMode{}
	payload := `{"jsonrpc":"2.0","id":0,"method":"initialize"}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)
	reader := newMCPAutoReader(strings.NewReader(input), mode)
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(out) != payload+"\n" {
		t.Fatalf("unexpected output: %q", string(out))
	}
	if got := mode.get(); got != mcpWireContentLength {
		t.Fatalf("expected content-length mode, got %v", got)
	}
}

func TestMCPAutoReader_RawJSONInitializeInput(t *testing.T) {
	mode := &mcpWireMode{}
	payload := `{"jsonrpc":"2.0","id":0,"method":"initialize"}` + "\n"
	reader := newMCPAutoReader(strings.NewReader(payload), mode)
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(out) != payload {
		t.Fatalf("unexpected output: %q", string(out))
	}
	if got := mode.get(); got != mcpWireNDJSON {
		t.Fatalf("expected ndjson mode, got %v", got)
	}
}

func TestMCPServe_InitializeFlow_EndToEnd(t *testing.T) {
	workspace := t.TempDir()
	srv := &mcpServer{workspaceRoot: workspace}
	result := performInitialize(t, srv, "2024-11-05")
	if _, ok := result["protocolVersion"].(string); !ok {
		t.Fatalf("initialize result missing protocolVersion: %#v", result)
	}
}

func TestMCPServe_InitializeFlow_NegotiatesSupportedProtocolVersion(t *testing.T) {
	workspace := t.TempDir()
	srv := &mcpServer{workspaceRoot: workspace}
	result := performInitialize(t, srv, "2024-11-05")
	if got := fmt.Sprintf("%v", result["protocolVersion"]); got != "2024-11-05" {
		t.Fatalf("expected supported protocol version to be preserved, got %q", got)
	}
}

func TestMCPServe_InitializeFlow_FallsBackForUnsupportedProtocolVersion(t *testing.T) {
	workspace := t.TempDir()
	srv := &mcpServer{workspaceRoot: workspace}
	result := performInitialize(t, srv, "2099-01-01")
	if got := fmt.Sprintf("%v", result["protocolVersion"]); got != latestNegotiatedProtocolVersion {
		t.Fatalf("expected fallback to latest supported protocol version %q, got %q", latestNegotiatedProtocolVersion, got)
	}
}

func TestMCPAutoWriter_NDJSONPassthroughAndFramedOutput(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":0,"result":{}}` + "\n"

	ndMode := &mcpWireMode{}
	var ndOut bytes.Buffer
	ndWriter := newMCPAutoWriter(&ndOut, ndMode)
	if _, err := ndWriter.Write([]byte(payload)); err != nil {
		t.Fatalf("ndjson write failed: %v", err)
	}
	if ndOut.String() != payload {
		t.Fatalf("expected passthrough payload, got %q", ndOut.String())
	}

	framedMode := &mcpWireMode{}
	framedMode.set(mcpWireContentLength)
	var framedOut bytes.Buffer
	framedWriter := newMCPAutoWriter(&framedOut, framedMode)
	if _, err := framedWriter.Write([]byte(payload)); err != nil {
		t.Fatalf("framed write failed: %v", err)
	}
	want := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(strings.TrimSpace(payload)), strings.TrimSpace(payload))
	if framedOut.String() != want {
		t.Fatalf("unexpected framed output:\n%s\nwant:\n%s", framedOut.String(), want)
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

func TestMCPDefineResourceSchema_UpdatesTypeFile(t *testing.T) {
	workspace := t.TempDir()
	projectDir := filepath.Join(workspace, "schema-api")
	opts := &initOptions{
		modulePath:       "github.com/example/schema-api",
		withStorage:      true,
		withVersion:      true,
		validationMode:   "strict",
		eventBusType:     "memory",
		apiGroup:         "example.fabrica.dev",
		storageVersion:   "v1",
		apiVersions:      []string{"v1"},
		storageType:      "file",
		dbDriver:         "sqlite",
		reconcileWorkers: 3,
	}
	if err := runInit(projectDir, opts); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	if err := withWorkingDir(projectDir, func() error {
		return runAddResource("Widget", &addOptions{withValidation: true, withStatus: true})
	}); err != nil {
		t.Fatalf("runAddResource failed: %v", err)
	}

	srv := &mcpServer{workspaceRoot: workspace}
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

func readFramedMCPResponse(r io.Reader) ([]byte, error) {
	br := bufio.NewReader(r)
	contentLength := -1
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(br, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func performInitialize(t *testing.T, srv *mcpServer, protocolVersion string) map[string]interface{} {
	t.Helper()

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		errCh <- srv.serveWithIO(inR, outW)
	}()

	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "mcp-test",
				"version": "0.0.0",
			},
		},
	}
	payload, err := json.Marshal(initReq)
	if err != nil {
		t.Fatalf("marshal initialize payload: %v", err)
	}
	framed := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)
	if _, err := io.WriteString(inW, framed); err != nil {
		t.Fatalf("write initialize request: %v", err)
	}

	respPayload, err := readFramedMCPResponse(outR)
	if err != nil {
		t.Fatalf("read initialize response: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(respPayload, &resp); err != nil {
		t.Fatalf("unmarshal initialize response: %v", err)
	}
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("initialize returned error: %s", string(respPayload))
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("initialize response missing result: %s", string(respPayload))
	}

	_ = inW.Close()
	if err := <-errCh; err != nil {
		t.Fatalf("serveWithIO exited with error: %v", err)
	}

	return result
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewCommand creates the "fabrica mcp" command for running as an MCP server over stdio.
// The server validates that all operations remain within the specified workspace.
func NewCommand() *cobra.Command {
	var workspace string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run Fabrica as an MCP server over stdio",
		Long: `Run Fabrica as a local MCP server over stdio.

The server supports both dry-run and execute modes for mutating tools and keeps
all operations constrained to the workspace root.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := resolveWorkspaceRoot(workspace)
			if err != nil {
				return err
			}

			server := &mcpServer{
				workspaceRoot: root,
				in:            bufio.NewReader(os.Stdin),
				out:           os.Stdout,
			}
			return server.serve()
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", ".", "Workspace root for all MCP operations")

	return cmd
}

// MCP Protocol Server Implementation

// serve handles the MCP JSON-RPC message loop over stdio.
// Reads messages, processes them, and writes responses.
// Terminates cleanly on EOF or fatal errors.
func (s *mcpServer) serve() error {
	for {
		payload, err := readMCPMessage(s.in.(*bufio.Reader))
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		var req mcpRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			if err := s.writeError(nil, -32700, "Parse error", err.Error()); err != nil {
				return err
			}
			continue
		}

		if req.Method == "" {
			if err := s.writeError(req.idValue(), -32600, "Invalid request", "missing method"); err != nil {
				return err
			}
			continue
		}

		result, callErr := s.handle(req)
		if len(req.ID) == 0 {
			// Notification, no response
			continue
		}
		if callErr != nil {
			if err := s.writeError(req.idValue(), -32000, callErr.Error(), nil); err != nil {
				return err
			}
			continue
		}
		if err := s.writeResult(req.idValue(), result); err != nil {
			return err
		}
	}
}

// handle dispatches incoming MCP requests to appropriate handlers.
// Supports initialize, initialized, tools/list, and tools/call methods.
func (s *mcpServer) handle(req mcpRequest) (interface{}, error) {
	switch req.Method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{"listChanged": false},
			},
			"serverInfo": map[string]interface{}{
				"name":    "fabrica-mcp",
				"version": Version,
			},
		}, nil
	case "notifications/initialized":
		return nil, nil
	case "tools/list":
		return map[string]interface{}{"tools": s.tools()}, nil
	case "tools/call":
		var params mcpToolCallParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, fmt.Errorf("invalid tools/call params: %w", err)
			}
		}
		return s.callTool(params)
	default:
		return nil, fmt.Errorf("method not found: %s", req.Method)
	}
}

// tools returns the list of available MCP tools.
// Each tool includes name, description, and input schema.
func (s *mcpServer) tools() []mcpToolDef {
	return []mcpToolDef{
		{
			Name:        "inspect_project",
			Description: "Inspect Fabrica project configuration, API versions, resources, and enabled features",
			InputSchema: schemaObject(map[string]interface{}{
				"project_path": map[string]interface{}{"type": "string", "description": "Project path relative to workspace root"},
			}),
		},
		{
			Name:        "validate_project",
			Description: "Validate Fabrica project structure and configuration consistency",
			InputSchema: schemaObject(map[string]interface{}{
				"project_path": map[string]interface{}{"type": "string", "description": "Project path relative to workspace root"},
			}),
		},
		{
			Name:        "create_service",
			Description: "Create a new Fabrica service project (dry_run or execute)",
			InputSchema: schemaObjectWithRequired(map[string]interface{}{
				"project_name":    map[string]interface{}{"type": "string"},
				"mode":            modeField(),
				"target_dir":      map[string]interface{}{"type": "string", "description": "Directory where project should be created"},
				"module":          map[string]interface{}{"type": "string"},
				"description":     map[string]interface{}{"type": "string"},
				"group":           map[string]interface{}{"type": "string"},
				"storage_version": map[string]interface{}{"type": "string"},
				"versions": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
				"auth":              map[string]interface{}{"type": "boolean"},
				"metrics":           map[string]interface{}{"type": "boolean"},
				"events":            map[string]interface{}{"type": "boolean"},
				"events_bus":        map[string]interface{}{"type": "string", "enum": []string{"memory"}},
				"reconcile":         map[string]interface{}{"type": "boolean"},
				"reconcile_workers": map[string]interface{}{"type": "number"},
				"reconcile_requeue": map[string]interface{}{"type": "number"},
				"storage_type":      map[string]interface{}{"type": "string", "enum": []string{"file", "ent"}},
				"db":                map[string]interface{}{"type": "string", "enum": []string{"sqlite", "postgres", "mysql"}},
				"validation_mode":   map[string]interface{}{"type": "string", "enum": []string{"strict", "warn", "disabled"}},
			}, []string{"project_name"}),
		},
		{
			Name:        "add_resource",
			Description: "Add a new resource type to an existing Fabrica project (dry_run or execute)",
			InputSchema: schemaObjectWithRequired(map[string]interface{}{
				"resource_name":   map[string]interface{}{"type": "string"},
				"project_path":    map[string]interface{}{"type": "string"},
				"mode":            modeField(),
				"version":         map[string]interface{}{"type": "string"},
				"with_validation": map[string]interface{}{"type": "boolean"},
				"with_status":     map[string]interface{}{"type": "boolean"},
				"with_versioning": map[string]interface{}{"type": "boolean"},
				"package":         map[string]interface{}{"type": "string"},
				"force":           map[string]interface{}{"type": "boolean"},
			}, []string{"resource_name"}),
		},
		{
			Name:        "define_resource_schema",
			Description: "Replace a resource Spec/Status struct body from structured field definitions (dry_run or execute)",
			InputSchema: schemaObjectWithRequired(map[string]interface{}{
				"project_path":  map[string]interface{}{"type": "string"},
				"resource_name": map[string]interface{}{"type": "string"},
				"version":       map[string]interface{}{"type": "string"},
				"mode":          modeField(),
				"spec_fields":   fieldArraySchema(),
				"status_fields": fieldArraySchema(),
			}, []string{"resource_name"}),
		},
		{
			Name:        "add_version",
			Description: "Add a new API version to an existing Fabrica project (dry_run or execute)",
			InputSchema: schemaObjectWithRequired(map[string]interface{}{
				"new_version":  map[string]interface{}{"type": "string"},
				"project_path": map[string]interface{}{"type": "string"},
				"mode":         modeField(),
				"from":         map[string]interface{}{"type": "string"},
				"force":        map[string]interface{}{"type": "boolean"},
			}, []string{"new_version"}),
		},
		{
			Name:        "generate_code",
			Description: "Generate Fabrica handlers/storage/client/openapi artifacts (dry_run or execute)",
			InputSchema: schemaObject(map[string]interface{}{
				"project_path": map[string]interface{}{"type": "string"},
				"mode":         modeField(),
				"artifacts": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string", "enum": []string{"handlers", "storage", "client", "openapi"}},
				},
				"force":          map[string]interface{}{"type": "boolean"},
				"debug":          map[string]interface{}{"type": "boolean"},
				"fabrica_source": map[string]interface{}{"type": "string"},
			}),
		},
		{
			Name:        "sync_dependencies",
			Description: "Run go mod tidy for a Fabrica project (dry_run or execute)",
			InputSchema: schemaObject(map[string]interface{}{
				"project_path": map[string]interface{}{"type": "string"},
				"mode":         modeField(),
			}),
		},
		{
			Name:        "build_project",
			Description: "Run go build for a Fabrica project (dry_run or execute)",
			InputSchema: schemaObject(map[string]interface{}{
				"project_path": map[string]interface{}{"type": "string"},
				"mode":         modeField(),
				"packages": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
			}),
		},
		{
			Name:        "test_project",
			Description: "Run go test for a Fabrica project (dry_run or execute)",
			InputSchema: schemaObject(map[string]interface{}{
				"project_path": map[string]interface{}{"type": "string"},
				"mode":         modeField(),
				"packages": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
			}),
		},
		{
			Name:        "smoke_test_api",
			Description: "Check generated API health and OpenAPI endpoints, optionally starting the server (dry_run or execute)",
			InputSchema: schemaObject(map[string]interface{}{
				"project_path":     map[string]interface{}{"type": "string"},
				"mode":             modeField(),
				"base_url":         map[string]interface{}{"type": "string"},
				"start_server":     map[string]interface{}{"type": "boolean"},
				"timeout_seconds":  map[string]interface{}{"type": "number"},
				"server_arguments": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			}),
		},
		{
			Name:        "describe_workflow",
			Description: "Return exact MCP tool-call sequences for common Fabrica REST API construction workflows",
			InputSchema: schemaObject(map[string]interface{}{
				"goal": map[string]interface{}{"type": "string", "enum": []string{"new_crud_api", "add_resource", "verify_project"}},
			}),
		},
	}
}

// callTool dispatches tool calls to their implementations.
// Validates arguments, executes the tool, and formats results.
func (s *mcpServer) callTool(call mcpToolCallParams) (interface{}, error) {
	args := call.Arguments
	if args == nil {
		args = map[string]interface{}{}
	}
	if err := validateMCPToolArgs(call.Name, args); err != nil {
		return toolErrorResult(err), nil
	}

	var (
		result map[string]interface{}
		err    error
	)

	switch call.Name {
	case "inspect_project":
		result, err = s.toolInspectProject(args)
	case "validate_project":
		result, err = s.toolValidateProject(args)
	case "create_service":
		result, err = s.toolCreateService(args)
	case "add_resource":
		result, err = s.toolAddResource(args)
	case "define_resource_schema":
		result, err = s.toolDefineResourceSchema(args)
	case "add_version":
		result, err = s.toolAddVersion(args)
	case "generate_code":
		result, err = s.toolGenerateCode(args)
	case "sync_dependencies":
		result, err = s.toolSyncDependencies(args)
	case "build_project":
		result, err = s.toolBuildProject(args)
	case "test_project":
		result, err = s.toolTestProject(args)
	case "smoke_test_api":
		result, err = s.toolSmokeTestAPI(args)
	case "describe_workflow":
		result, err = s.toolDescribeWorkflow(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", call.Name)
	}

	if err != nil {
		return toolErrorResult(err), nil
	}

	text, _ := json.MarshalIndent(result, "", "  ")
	return mcpToolCallResult{
		Content:           []mcpContent{{Type: "text", Text: string(text)}},
		StructuredContent: result,
	}, nil
}

// toolError creates a structured error response with remediation guidance.
func toolError(code, message, remediation string, err error) error {
	return &mcpToolError{
		Code:        code,
		Message:     message,
		Remediation: remediation,
		Err:         err,
	}
}

// toolErrorResult formats an error into an MCP tool call result.
func toolErrorResult(err error) mcpToolCallResult {
	toolErr := &mcpToolError{}
	code := "internal_error"
	message := err.Error()
	remediation := "Check tool inputs and retry."
	if errors.As(err, &toolErr) {
		if toolErr.Code != "" {
			code = toolErr.Code
		}
		if toolErr.Message != "" {
			message = toolErr.Message
		}
		if toolErr.Remediation != "" {
			remediation = toolErr.Remediation
		}
	}

	payload := map[string]interface{}{
		"status": "error",
		"error": map[string]interface{}{
			"code":        code,
			"message":     message,
			"remediation": remediation,
		},
	}
	if errText := err.Error(); errText != "" && errText != message {
		payload["error"].(map[string]interface{})["detail"] = errText
	}

	return mcpToolCallResult{
		Content:           []mcpContent{{Type: "text", Text: fmt.Sprintf("%s: %s", code, message)}},
		StructuredContent: payload,
		IsError:           true,
	}
}

// Message I/O

// readMCPMessage reads a single Content-Length delimited message from a Reader.
// Parses headers, validates Content-Length, and returns the message body.
func readMCPMessage(r *bufio.Reader) ([]byte, error) {
	headers := map[string]string{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}

	clHeader, ok := headers["content-length"]
	if !ok {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	length, err := strconv.Atoi(clHeader)
	if err != nil || length < 0 {
		return nil, fmt.Errorf("invalid Content-Length header")
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// writeResult sends a successful response to the client.
func (s *mcpServer) writeResult(id interface{}, result interface{}) error {
	resp := mcpResponse{JSONRPC: "2.0", ID: id, Result: result}
	return writeMCPMessage(s.out, resp)
}

// writeError sends an error response to the client.
func (s *mcpServer) writeError(id interface{}, code int, message string, data interface{}) error {
	resp := mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpError{Code: code, Message: message, Data: data}}
	return writeMCPMessage(s.out, resp)
}

// writeMCPMessage writes a message with Content-Length header and JSON body.
func writeMCPMessage(w io.Writer, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// idValue extracts the ID from an mcpRequest, returning nil if absent or invalid.
func (r mcpRequest) idValue() interface{} {
	if len(r.ID) == 0 {
		return nil
	}
	var decoded interface{}
	if err := json.Unmarshal(r.ID, &decoded); err != nil {
		return nil
	}
	return decoded
}

// HTTP and Context Helpers

// waitForHTTP polls a URL until it returns a 2xx status or context is canceled.
// Retries every 250ms up to the context deadline.
func waitForHTTP(ctx context.Context, url string) (int, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp.StatusCode, nil
			}
			lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return 0, lastErr
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// fileInfo returns the FileInfo for a path, wrapping os.Stat for brevity.
func fileInfo(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// Version is the MCP server version string.
var Version = "dev"

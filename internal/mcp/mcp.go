// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
				debug:         os.Getenv("FABRICA_MCP_DEBUG") == "1",
			}
			return server.serve()
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", ".", "Workspace root for all MCP operations")

	return cmd
}

// MCP Protocol Server Implementation

func (s *mcpServer) serve() error {
	return s.serveWithIO(os.Stdin, os.Stdout)
}

func (s *mcpServer) serveWithIO(in io.Reader, out io.Writer) error {
	serverVersion := Version
	if serverVersion == "" || serverVersion == "dev" {
		serverVersion = "0.0.0-dev"
	}

	sdkServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "fabrica-mcp",
			Version: serverVersion,
		},
		&mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{
				Resources: &mcp.ResourceCapabilities{ListChanged: false},
				Tools:     &mcp.ToolCapabilities{ListChanged: false},
			},
		},
	)

	s.registerTools(sdkServer)
	s.debugf("starting SDK-backed MCP server workspace=%s", s.workspaceRoot)
	wire := &mcpWireMode{}
	return sdkServer.Run(context.Background(), &mcp.IOTransport{
		Reader: newMCPAutoReader(in, wire),
		Writer: newMCPAutoWriter(out, wire),
	})
}

type mcpWireProtocol int

const (
	mcpWireUnknown mcpWireProtocol = iota
	mcpWireNDJSON
	mcpWireContentLength
)

type mcpWireMode struct {
	mu       sync.RWMutex
	protocol mcpWireProtocol
}

func (m *mcpWireMode) get() mcpWireProtocol {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.protocol
}

func (m *mcpWireMode) set(p mcpWireProtocol) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.protocol == mcpWireUnknown {
		m.protocol = p
	}
}

type mcpAutoReader struct {
	br      *bufio.Reader
	mode    *mcpWireMode
	pending []byte
}

func newMCPAutoReader(r io.Reader, mode *mcpWireMode) *mcpAutoReader {
	return &mcpAutoReader{
		br:   bufio.NewReader(r),
		mode: mode,
	}
}

func (r *mcpAutoReader) Read(p []byte) (int, error) {
	switch r.detectProtocol() {
	case mcpWireNDJSON:
		return r.br.Read(p)
	case mcpWireContentLength:
		if len(r.pending) == 0 {
			frame, err := r.readContentLengthFrame()
			if err != nil {
				return 0, err
			}
			r.pending = append(frame, '\n')
		}
		n := copy(p, r.pending)
		r.pending = r.pending[n:]
		return n, nil
	default:
		return 0, errors.New("unknown MCP wire protocol")
	}
}

func (r *mcpAutoReader) Close() error { return nil }

func (r *mcpAutoReader) detectProtocol() mcpWireProtocol {
	if p := r.mode.get(); p != mcpWireUnknown {
		return p
	}
	for {
		b, err := r.br.Peek(1)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return mcpWireUnknown
			}
			return mcpWireUnknown
		}
		switch b[0] {
		case ' ', '\t', '\r', '\n':
			_, _ = r.br.ReadByte()
			continue
		case '{', '[':
			r.mode.set(mcpWireNDJSON)
			return mcpWireNDJSON
		default:
			r.mode.set(mcpWireContentLength)
			return mcpWireContentLength
		}
	}
}

func (r *mcpAutoReader) readContentLengthFrame() ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.br.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return nil, io.EOF
			}
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
			n, convErr := strconv.Atoi(strings.TrimSpace(value))
			if convErr != nil || n < 0 {
				return nil, fmt.Errorf("invalid Content-Length header %q", line)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, errors.New("missing Content-Length header")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(r.br, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

type mcpAutoWriter struct {
	out  io.Writer
	mode *mcpWireMode

	mu  sync.Mutex
	buf []byte
}

func newMCPAutoWriter(w io.Writer, mode *mcpWireMode) *mcpAutoWriter {
	return &mcpAutoWriter{
		out:  w,
		mode: mode,
	}
}

func (w *mcpAutoWriter) Write(p []byte) (int, error) {
	if w.mode.get() != mcpWireContentLength {
		_, err := w.out.Write(p)
		return len(p), err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := bytes.TrimSpace(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		if len(line) == 0 {
			continue
		}
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(line))
		if os.Getenv("FABRICA_MCP_DEBUG") == "1" {
			_, _ = fmt.Fprintf(os.Stderr, "[fabrica-mcp] outgoing header=%q payload=%s\n", header, string(line))
		}
		if _, err := io.WriteString(w.out, header); err != nil {
			return 0, err
		}
		if _, err := w.out.Write(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *mcpAutoWriter) Close() error {
	if w.mode.get() != mcpWireContentLength {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(bytes.TrimSpace(w.buf)) > 0 {
		line := bytes.TrimSpace(w.buf)
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(line))
		if _, err := io.WriteString(w.out, header); err != nil {
			return err
		}
		if _, err := w.out.Write(line); err != nil {
			return err
		}
	}
	w.buf = nil
	return nil
}

func (s *mcpServer) registerTools(sdkServer *mcp.Server) {
	for _, def := range s.tools() {
		toolDef := def
		sdkServer.AddTool(
			&mcp.Tool{
				Name:        toolDef.Name,
				Title:       toolDef.Name,
				Description: toolDef.Description,
				InputSchema: toolDef.InputSchema,
			},
			func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_ = ctx
				args := map[string]interface{}{}
				if len(req.Params.Arguments) > 0 {
					if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
						result := &mcp.CallToolResult{}
						result.SetError(fmt.Errorf("invalid tool arguments JSON: %w", err))
						return result, nil
					}
				}
				return s.callToolSDK(toolDef.Name, args)
			},
		)
	}
}

func (s *mcpServer) callToolSDK(name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	callRes, err := s.callTool(mcpToolCallParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	res, ok := callRes.(mcpToolCallResult)
	if !ok {
		return nil, fmt.Errorf("unexpected tool result type %T", callRes)
	}

	content := make([]mcp.Content, 0, len(res.Content))
	for _, c := range res.Content {
		if c.Type == "text" {
			content = append(content, &mcp.TextContent{Text: c.Text})
		}
	}
	if len(content) == 0 {
		content = append(content, &mcp.TextContent{Text: "{}"})
	}

	return &mcp.CallToolResult{
		Content:           content,
		StructuredContent: res.StructuredContent,
		IsError:           res.IsError,
	}, nil
}

func (s *mcpServer) debugf(format string, args ...interface{}) {
	if !s.debug {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "[fabrica-mcp] "+format+"\n", args...)
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

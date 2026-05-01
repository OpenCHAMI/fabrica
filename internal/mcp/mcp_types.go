// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"encoding/json"
	"io"
)

// mcpServer represents an MCP (Model Context Protocol) server that communicates over stdio.
// It manages workspace boundaries and handles all tool calls within the workspace root.
type mcpServer struct {
	workspaceRoot string
	in            io.Reader
	out           io.Writer
}

// MCP Protocol Message Types

// mcpRequest represents an incoming MCP JSON-RPC request.
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcpResponse represents an outgoing MCP JSON-RPC response.
type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

// mcpError represents an error in an MCP JSON-RPC response.
type mcpError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool Definition and Invocation Types

// mcpToolDef describes an MCP tool that can be called by the client.
type mcpToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// mcpToolCallParams represents the parameters to a tool call.
type mcpToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// mcpToolCallResult represents the result of a tool call.
type mcpToolCallResult struct {
	Content           []mcpContent           `json:"content"`
	StructuredContent map[string]interface{} `json:"structuredContent,omitempty"`
	IsError           bool                   `json:"isError,omitempty"`
}

// mcpContent represents a single content block in a tool call result.
type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Resource Schema Types

// mcpResourceField represents a single field in a Spec or Status struct.
type mcpResourceField struct {
	Name        string // Go field name (exported)
	Type        string // Go type expression (e.g., "string", "*int", "[]MyType")
	JSONName    string // JSON tag name (auto-derived from Name if not set)
	Required    bool   // Whether field is required
	Validation  string // Validation constraint tag (e.g., "required", "max=100")
	Description string // Go comment documentation
}

// Error Types

// mcpToolError represents a detailed error from a tool execution, with recovery guidance.
type mcpToolError struct {
	Code        string // Machine-readable error code (e.g., "workspace_violation")
	Message     string // Human-readable error message
	Remediation string // Suggested action to fix the error
	Err         error  // Underlying error
}

// Error() returns the string representation of the error.
func (e *mcpToolError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "mcp tool error"
}

// Unwrap returns the underlying error for error chain traversal.
func (e *mcpToolError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

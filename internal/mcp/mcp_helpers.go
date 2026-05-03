// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package mcp provides Fabrica's built-in MCP server and supporting helpers.
package mcp

import (
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	configpkg "github.com/openchami/fabrica/internal/config"
)

// Argument Extraction Helpers
// These functions safely extract typed values from MCP tool arguments with defaults.

// getString extracts a string argument, returning a default if not found or wrong type.
func getString(args map[string]interface{}, key, def string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

// getBool extracts a boolean argument, returning a default if not found or wrong type.
func getBool(args map[string]interface{}, key string, def bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

// getNumber extracts a numeric argument, returning a default if not found or wrong type.
func getNumber(args map[string]interface{}, key string, def int) int {
	raw, ok := args[key]
	if !ok {
		return def
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return def
	}
}

// getStringArray extracts an array of strings, returning a default if not found or wrong type.
func getStringArray(args map[string]interface{}, key string, def []string) []string {
	raw, ok := args[key]
	if !ok {
		return def
	}
	items, ok := raw.([]interface{})
	if !ok {
		if strings, ok := raw.([]string); ok {
			return strings
		}
		return def
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// getMode extracts the mode argument, defaulting to "dry_run".
func getMode(args map[string]interface{}) string {
	mode := getString(args, "mode", "dry_run")
	if mode == "execute" {
		return "execute"
	}
	return "dry_run"
}

// Path and Workspace Helpers

// resolveProjectPath resolves a project path relative to the workspace root,
// ensuring the result stays within workspace boundaries.
func (s *mcpServer) resolveProjectPath(input string) (string, error) {
	if input == "" {
		input = "."
	}
	candidate := input
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(s.workspaceRoot, input)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	root := filepath.Clean(s.workspaceRoot)
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", toolError("workspace_violation", "path is outside workspace root", "Pass a path that is inside --workspace", fmt.Errorf("path %s is outside workspace root %s", abs, root))
	}
	return abs, nil
}

// resolveWorkspaceRoot validates and returns an absolute path to the workspace root.
func resolveWorkspaceRoot(path string) (string, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", toolError("invalid_workspace", "workspace must be a directory", "Pass an existing directory path to --workspace", fmt.Errorf("workspace must be a directory: %s", abs))
	}
	return abs, nil
}

// String Utilities

// lowerFirst converts the first character of a string to lowercase.
func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// toResourceName converts a snake_case identifier to PascalCase.
func toResourceName(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "_")
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

// truncateOutput limits output to maxLen characters with truncation message.
func truncateOutput(output string) string {
	output = strings.TrimSpace(output)
	const maxLen = 8000
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "\n... output truncated ..."
}

// Collection Utilities

// contains reports whether item is in items.
func contains(items []string, item string) bool {
	for _, it := range items {
		if it == item {
			return true
		}
	}
	return false
}

// File Operations

// listResourceTypeFiles lists resource names (converted from *_types.go files) in a version directory.
func listResourceTypeFiles(versionDir string) ([]string, error) {
	entries, err := os.ReadDir(versionDir)
	if err != nil {
		return nil, err
	}
	resources := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, "_types.go") {
			continue
		}
		base := strings.TrimSuffix(name, "_types.go")
		if base == "" {
			continue
		}
		resources = append(resources, toResourceName(base))
	}
	sort.Strings(resources)
	return resources, nil
}

// listGeneratedFiles finds all *_generated.go files under projectDir.
func listGeneratedFiles(projectDir string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(projectDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), "_generated.go") {
			rel, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// mustListGeneratedFiles is like listGeneratedFiles but returns empty slice on error.
func mustListGeneratedFiles(projectDir string) []string {
	files, err := listGeneratedFiles(projectDir)
	if err != nil {
		return []string{}
	}
	return files
}

// latestModTimeForPattern finds the file matching the glob pattern with the latest modification time.
// Supports ** for recursive matching.
func latestModTimeForPattern(globPattern string) (time.Time, string, error) {
	if strings.Contains(globPattern, "**") {
		root := strings.Split(globPattern, "**")[0]
		if root == "" {
			root = "."
		}
		var latest time.Time
		var latestPath string
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, "_types.go") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.ModTime().After(latest) {
				latest = info.ModTime()
				latestPath = path
			}
			return nil
		})
		if err != nil {
			return time.Time{}, "", err
		}
		if latest.IsZero() {
			return time.Time{}, "", fmt.Errorf("no files matched pattern")
		}
		return latest, latestPath, nil
	}

	paths, err := filepath.Glob(globPattern)
	if err != nil {
		return time.Time{}, "", err
	}
	var latest time.Time
	var latestPath string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
			latestPath = p
		}
	}
	if latest.IsZero() {
		return time.Time{}, "", fmt.Errorf("no files matched pattern")
	}
	return latest, latestPath, nil
}

// Process Execution

// withWorkingDir temporarily changes to dir, executes fn, then restores the original directory.
func withWorkingDir(dir string, fn func() error) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()
	return fn()
}

// execCommand runs a command in a working directory with output capture.
// Returns combined stdout/stderr output and any execution error.
func execCommand(dir, command string, args []string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Code Generation Helpers

// renderMCPFields generates Go struct field declarations from mcpResourceField definitions.
// When includeValidation is true, adds validation tags for required fields.
func renderMCPFields(fields []mcpResourceField, includeValidation bool) string {
	var b strings.Builder
	for _, field := range fields {
		if field.Description != "" {
			b.WriteString("\t// " + field.Description + "\n")
		}
		jsonTag := field.JSONName
		if !field.Required {
			jsonTag += ",omitempty"
		}
		tagParts := []string{fmt.Sprintf(`json:"%s"`, jsonTag)}
		validation := field.Validation
		if includeValidation && field.Required && validation == "" {
			validation = "required"
		}
		if includeValidation && validation != "" {
			tagParts = append(tagParts, fmt.Sprintf(`validate:"%s"`, validation))
		}
		fmt.Fprintf(&b, "\t%s %s `%s`\n", field.Name, field.Type, strings.Join(tagParts, " "))
	}
	if len(fields) == 0 {
		b.WriteString("\t// Add fields here\n")
	}
	return b.String()
}

// replaceStructBody finds and replaces the body of a named struct in Go code.
// Preserves formatting and returns the modified code.
func replaceStructBody(content, typeName, body string) (string, error) {
	needle := "type " + typeName + " struct {"
	start := strings.Index(content, needle)
	if start < 0 {
		return "", fmt.Errorf("struct %s not found", typeName)
	}
	open := start + len(needle) - 1
	depth := 0
	for i := open; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[:open+1] + "\n" + body + content[i:], nil
			}
		}
	}
	return "", fmt.Errorf("struct %s closing brace not found", typeName)
}

// rewriteResourceSchema rewrites Spec and Status struct field bodies in a resource file.
// Formats the file with go/format after modification.
func rewriteResourceSchema(filePath, resourceName string, specFields, statusFields []mcpResourceField) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)
	if len(specFields) > 0 {
		updated, err := replaceStructBody(content, resourceName+"Spec", renderMCPFields(specFields, true))
		if err != nil {
			return err
		}
		content = updated
	}
	if len(statusFields) > 0 {
		updated, err := replaceStructBody(content, resourceName+"Status", renderMCPFields(statusFields, false))
		if err != nil {
			return err
		}
		content = updated
	}
	formatted, err := format.Source([]byte(content))
	if err != nil {
		return fmt.Errorf("format updated resource file: %w", err)
	}
	return os.WriteFile(filePath, formatted, 0o644)
}

// Resource and Endpoint Mapping

// resourceFileMap creates a mapping from resource names to their type file paths.
func resourceFileMap(projectDir string, group *configpkg.APIGroup) map[string]string {
	files := map[string]string{}
	if group == nil {
		return files
	}
	for _, resource := range group.Resources {
		files[resource] = filepath.ToSlash(filepath.Join(projectDir, "apis", group.Name, group.StorageVersion, strings.ToLower(resource)+"_types.go"))
	}
	return files
}

// resourceEndpoints generates standard REST endpoint paths for a list of resources.
func resourceEndpoints(resources []string) map[string][]string {
	out := make(map[string][]string, len(resources))
	for _, resource := range resources {
		base := "/" + strings.ToLower(resource) + "s"
		out[resource] = []string{
			"GET " + base,
			"POST " + base,
			"GET " + base + "/{uid}",
			"PUT " + base + "/{uid}",
			"PATCH " + base + "/{uid}",
			"DELETE " + base + "/{uid}",
			"PUT " + base + "/{uid}/status",
			"PATCH " + base + "/{uid}/status",
		}
	}
	return out
}

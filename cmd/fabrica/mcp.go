// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newMCPCommand() *cobra.Command {
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

type mcpServer struct {
	workspaceRoot string
	in            *bufio.Reader
	out           io.Writer
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type mcpToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type mcpToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type mcpToolCallResult struct {
	Content           []mcpContent           `json:"content"`
	StructuredContent map[string]interface{} `json:"structuredContent,omitempty"`
	IsError           bool                   `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolError struct {
	Code        string
	Message     string
	Remediation string
	Err         error
}

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

func (e *mcpToolError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func toolError(code, message, remediation string, err error) error {
	return &mcpToolError{
		Code:        code,
		Message:     message,
		Remediation: remediation,
		Err:         err,
	}
}

func (s *mcpServer) serve() error {
	for {
		payload, err := readMCPMessage(s.in)
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
			// Notification, no response.
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
				"version": version,
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
				"auth":            map[string]interface{}{"type": "boolean"},
				"metrics":         map[string]interface{}{"type": "boolean"},
				"events":          map[string]interface{}{"type": "boolean"},
				"events_bus":      map[string]interface{}{"type": "string", "enum": []string{"memory", "nats", "kafka"}},
				"storage_type":    map[string]interface{}{"type": "string", "enum": []string{"file", "ent"}},
				"db":              map[string]interface{}{"type": "string", "enum": []string{"sqlite", "postgres", "mysql"}},
				"validation_mode": map[string]interface{}{"type": "string", "enum": []string{"strict", "warn", "disabled"}},
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
	}
}

func (s *mcpServer) callTool(call mcpToolCallParams) (interface{}, error) {
	args := call.Arguments
	if args == nil {
		args = map[string]interface{}{}
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
	case "add_version":
		result, err = s.toolAddVersion(args)
	case "generate_code":
		result, err = s.toolGenerateCode(args)
	case "sync_dependencies":
		result, err = s.toolSyncDependencies(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", call.Name)
	}

	if err != nil {
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

		return mcpToolCallResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("%s: %s", code, message)}},
			StructuredContent: map[string]interface{}{
				"status": "error",
				"error": map[string]interface{}{
					"code":        code,
					"message":     message,
					"remediation": remediation,
				},
			},
			IsError: true,
		}, nil
	}

	text, _ := json.MarshalIndent(result, "", "  ")
	return mcpToolCallResult{
		Content:           []mcpContent{{Type: "text", Text: string(text)}},
		StructuredContent: result,
	}, nil
}

func (s *mcpServer) toolInspectProject(args map[string]interface{}) (map[string]interface{}, error) {
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project path is outside workspace", "Provide a project_path within --workspace", err)
	}

	var cfg *FabricaConfig
	var apis *APIsConfig
	var resources []string

	err = withWorkingDir(projectDir, func() error {
		var loadErr error
		cfg, loadErr = LoadConfig("")
		if loadErr != nil {
			return toolError("missing_config", "failed to load .fabrica.yaml", "Run fabrica init in the project root or set project_path correctly", loadErr)
		}
		apis, loadErr = LoadAPIsConfig("")
		if loadErr != nil {
			return toolError("missing_apis_config", "failed to load apis.yaml", "Ensure apis.yaml exists in project root", loadErr)
		}
		resources, loadErr = discoverResources(apis)
		return loadErr
	})
	if err != nil {
		return nil, err
	}

	group, err := apis.primaryGroup()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":          "ok",
		"project_path":    projectDir,
		"project_name":    cfg.Project.Name,
		"module":          cfg.Project.Module,
		"storage_type":    cfg.Features.Storage.Type,
		"db_driver":       cfg.Features.Storage.DBDriver,
		"api_group":       group.Name,
		"storage_version": group.StorageVersion,
		"versions":        group.Versions,
		"resources":       resources,
		"features": map[string]interface{}{
			"validation_mode": cfg.Features.Validation.Mode,
			"events":          cfg.Features.Events.Enabled,
			"auth":            cfg.Features.Auth.Enabled,
			"reconciliation":  cfg.Features.Reconciliation.Enabled,
		},
	}, nil
}

func (s *mcpServer) toolValidateProject(args map[string]interface{}) (map[string]interface{}, error) {
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project path is outside workspace", "Provide a project_path within --workspace", err)
	}

	issues := make([]map[string]interface{}, 0)
	addIssue := func(severity, code, message string) {
		issues = append(issues, map[string]interface{}{"severity": severity, "code": code, "message": message})
	}

	cfgPath := filepath.Join(projectDir, ConfigFileName)
	if _, err := os.Stat(cfgPath); err != nil {
		addIssue("error", "missing_config", fmt.Sprintf("missing %s", ConfigFileName))
	}
	apisPath := filepath.Join(projectDir, APIsConfigFileName)
	if _, err := os.Stat(apisPath); err != nil {
		addIssue("error", "missing_apis_config", fmt.Sprintf("missing %s", APIsConfigFileName))
	}

	if len(issues) == 0 {
		err = withWorkingDir(projectDir, func() error {
			cfg, loadErr := LoadConfig("")
			if loadErr != nil {
				return loadErr
			}
			if validateErr := ValidateConfig(cfg); validateErr != nil {
				addIssue("error", "invalid_config", validateErr.Error())
			}

			apis, loadErr := LoadAPIsConfig("")
			if loadErr != nil {
				return loadErr
			}
			if validateErr := apis.Validate(); validateErr != nil {
				addIssue("error", "invalid_apis_config", validateErr.Error())
			}

			group, groupErr := apis.primaryGroup()
			if groupErr != nil {
				addIssue("error", "invalid_primary_group", groupErr.Error())
				return nil
			}

			for _, v := range group.Versions {
				versionDir := filepath.Join("apis", group.Name, v)
				if _, statErr := os.Stat(versionDir); statErr != nil {
					addIssue("warning", "missing_version_dir", fmt.Sprintf("version directory not found: %s", versionDir))
				}
			}

			storageResourceFiles, fileErr := listResourceTypeFiles(filepath.Join("apis", group.Name, group.StorageVersion))
			if fileErr == nil {
				storageResources := make(map[string]struct{}, len(storageResourceFiles))
				for _, name := range storageResourceFiles {
					storageResources[name] = struct{}{}
				}

				for _, resource := range group.Resources {
					if _, ok := storageResources[resource]; !ok {
						addIssue("warning", "resource_list_drift", fmt.Sprintf("resource %s listed in apis.yaml but not found in storage version dir", resource))
					}
				}
				for _, resource := range storageResourceFiles {
					if !contains(group.Resources, resource) {
						addIssue("warning", "resource_list_drift", fmt.Sprintf("resource %s exists in storage version dir but is missing from apis.yaml resources", resource))
					}
				}
			}

			resources, discErr := discoverResources(apis)
			if discErr != nil {
				addIssue("error", "resource_discovery_failed", discErr.Error())
			} else if len(resources) == 0 {
				addIssue("warning", "no_resources", "no resources found; run fabrica add resource <name>")
			}

			routesPath := filepath.Join("cmd", "server", "routes_generated.go")
			routesInfo, statErr := os.Stat(routesPath)
			if statErr != nil {
				addIssue("warning", "missing_generated_artifacts", "generated server artifacts not found; run fabrica generate")
			} else {
				latestType, latestTypePath, latestErr := latestModTimeForPattern(filepath.Join("apis", "**", "*_types.go"))
				if latestErr == nil && latestType.After(routesInfo.ModTime()) {
					addIssue("warning", "generated_artifacts_stale", fmt.Sprintf("%s is newer than %s; run fabrica generate", filepath.ToSlash(latestTypePath), filepath.ToSlash(routesPath)))
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	status := "ok"
	for _, issue := range issues {
		if issue["severity"] == "error" {
			status = "error"
			break
		}
	}
	if status == "ok" && len(issues) > 0 {
		status = "warning"
	}

	return map[string]interface{}{
		"status":       status,
		"project_path": projectDir,
		"issues":       issues,
	}, nil
}

func (s *mcpServer) toolCreateService(args map[string]interface{}) (map[string]interface{}, error) {
	projectName := strings.TrimSpace(getString(args, "project_name", ""))
	if projectName == "" {
		return nil, toolError("invalid_arguments", "project_name is required", "Provide a non-empty project_name", nil)
	}
	mode := getMode(args)
	targetDir, err := s.resolveProjectPath(getString(args, "target_dir", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "target_dir is outside workspace", "Provide a target_dir within --workspace", err)
	}

	opts := &initOptions{
		withStorage:    true,
		withVersion:    true,
		storageType:    getString(args, "storage_type", "file"),
		dbDriver:       getString(args, "db", "sqlite"),
		validationMode: getString(args, "validation_mode", "strict"),
		eventBusType:   getString(args, "events_bus", "memory"),
		apiGroup:       getString(args, "group", ""),
		storageVersion: getString(args, "storage_version", "v1"),
		modulePath:     getString(args, "module", ""),
		description:    getString(args, "description", ""),
		withAuth:       getBool(args, "auth", false),
		withMetrics:    getBool(args, "metrics", false),
		withEvents:     getBool(args, "events", false),
	}
	if versions, ok := args["versions"].([]interface{}); ok && len(versions) > 0 {
		opts.apiVersions = make([]string, 0, len(versions))
		for _, v := range versions {
			opts.apiVersions = append(opts.apiVersions, fmt.Sprintf("%v", v))
		}
	} else {
		opts.apiVersions = []string{"v1"}
	}

	if mode == "dry_run" {
		createdPath := projectName
		if projectName == "." {
			createdPath = targetDir
		} else {
			createdPath = filepath.Join(targetDir, projectName)
		}
		plannedFiles := predictCreateServiceFiles(createdPath)
		return map[string]interface{}{
			"status":        "dry_run",
			"project_name":  projectName,
			"target_dir":    targetDir,
			"planned_files": plannedFiles,
			"planned_steps": []string{
				"initialize project scaffold",
				"create .fabrica.yaml",
				"create apis.yaml",
				"create cmd/server and internal/storage scaffolding",
			},
		}, nil
	}

	err = withWorkingDir(targetDir, func() error {
		return runWithSilencedOutput(func() error {
			return runInit(projectName, opts)
		})
	})
	if err != nil {
		return nil, toolError("create_service_failed", "failed to create Fabrica service", "Check if target path exists and is writable", err)
	}

	createdPath := projectName
	if projectName == "." {
		createdPath = targetDir
	} else {
		createdPath = filepath.Join(targetDir, projectName)
	}

	return map[string]interface{}{
		"status":         "ok",
		"project_name":   projectName,
		"created_path":   createdPath,
		"next_steps":     []string{"add_resource", "generate_code", "sync_dependencies"},
		"execution_mode": mode,
	}, nil
}

func (s *mcpServer) toolAddResource(args map[string]interface{}) (map[string]interface{}, error) {
	resourceName := strings.TrimSpace(getString(args, "resource_name", ""))
	if resourceName == "" {
		return nil, toolError("invalid_arguments", "resource_name is required", "Provide a non-empty resource_name", nil)
	}
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project_path is outside workspace", "Provide a project_path within --workspace", err)
	}
	mode := getMode(args)

	opts := &addOptions{
		withValidation: getBool(args, "with_validation", true),
		withStatus:     getBool(args, "with_status", true),
		withVersioning: getBool(args, "with_versioning", false),
		packageName:    getString(args, "package", ""),
		version:        getString(args, "version", ""),
		force:          getBool(args, "force", false),
	}

	if mode == "dry_run" {
		planned, predErr := predictAddResourceFiles(projectDir, resourceName, opts.version)
		if predErr != nil {
			planned = []string{filepath.ToSlash(filepath.Join("apis", "<group>", "<version>", strings.ToLower(resourceName)+"_types.go")), APIsConfigFileName}
		}
		return map[string]interface{}{
			"status":        "dry_run",
			"resource_name": resourceName,
			"project_path":  projectDir,
			"planned_files": planned,
			"planned_steps": []string{
				"create resource type file under apis/<group>/<version>/",
				"update apis.yaml resource list",
			},
		}, nil
	}

	err = withWorkingDir(projectDir, func() error {
		return runWithSilencedOutput(func() error {
			return runAddResource(resourceName, opts)
		})
	})
	if err != nil {
		return nil, toolError("add_resource_failed", "failed to add resource", "Ensure project has valid .fabrica.yaml and apis.yaml", err)
	}

	return map[string]interface{}{
		"status":        "ok",
		"resource_name": resourceName,
		"project_path":  projectDir,
		"next_steps":    []string{"generate_code", "sync_dependencies"},
	}, nil
}

func (s *mcpServer) toolAddVersion(args map[string]interface{}) (map[string]interface{}, error) {
	newVersion := strings.TrimSpace(getString(args, "new_version", ""))
	if newVersion == "" {
		return nil, toolError("invalid_arguments", "new_version is required", "Provide a non-empty new_version", nil)
	}
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project_path is outside workspace", "Provide a project_path within --workspace", err)
	}
	mode := getMode(args)

	opts := &versionOptions{
		from:  getString(args, "from", ""),
		force: getBool(args, "force", false),
	}

	if mode == "dry_run" {
		planned, predErr := predictAddVersionFiles(projectDir, newVersion, opts.from)
		if predErr != nil {
			planned = []string{filepath.ToSlash(filepath.Join("apis", "<group>", newVersion, "*_types.go")), APIsConfigFileName}
		}
		return map[string]interface{}{
			"status":        "dry_run",
			"new_version":   newVersion,
			"project_path":  projectDir,
			"planned_files": planned,
			"planned_steps": []string{
				"create new version directory under apis/<group>/",
				"copy *_types.go files from source version",
				"append version to apis.yaml",
			},
		}, nil
	}

	err = withWorkingDir(projectDir, func() error {
		return runWithSilencedOutput(func() error {
			return runAddVersion(newVersion, opts)
		})
	})
	if err != nil {
		return nil, toolError("add_version_failed", "failed to add API version", "Ensure project has valid apis.yaml and source version exists", err)
	}

	return map[string]interface{}{
		"status":       "ok",
		"new_version":  newVersion,
		"project_path": projectDir,
		"next_steps":   []string{"generate_code", "sync_dependencies"},
	}, nil
}

func (s *mcpServer) toolGenerateCode(args map[string]interface{}) (map[string]interface{}, error) {
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project_path is outside workspace", "Provide a project_path within --workspace", err)
	}
	mode := getMode(args)

	artifactFlags := make([]string, 0)
	if raw, ok := args["artifacts"].([]interface{}); ok {
		for _, item := range raw {
			switch fmt.Sprintf("%v", item) {
			case "handlers":
				artifactFlags = append(artifactFlags, "--handlers")
			case "storage":
				artifactFlags = append(artifactFlags, "--storage")
			case "client":
				artifactFlags = append(artifactFlags, "--client")
			case "openapi":
				artifactFlags = append(artifactFlags, "--openapi")
			}
		}
	}
	if getBool(args, "debug", false) {
		artifactFlags = append(artifactFlags, "--debug")
	}
	if getBool(args, "force", false) {
		artifactFlags = append(artifactFlags, "--force")
	}
	if src := getString(args, "fabrica_source", ""); src != "" {
		artifactFlags = append(artifactFlags, "--fabrica-source", src)
	}

	if mode == "dry_run" {
		predicted := predictGenerateImpact(projectDir, artifactFlags)
		return map[string]interface{}{
			"status":        "dry_run",
			"project_path":  projectDir,
			"flags":         artifactFlags,
			"planned_files": predicted,
			"planned_steps": []string{
				"discover resources",
				"refresh registration",
				"generate server/client/storage/openapi artifacts",
			},
		}, nil
	}

	err = withWorkingDir(projectDir, func() error {
		return runWithSilencedOutput(func() error {
			cmd := newGenerateCommand()
			cmd.SetArgs(artifactFlags)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return cmd.Execute()
		})
	})
	if err != nil {
		return nil, toolError("generate_failed", "fabrica generate failed", "Inspect resource definitions and retry, or use debug=true", err)
	}

	generated, listErr := listGeneratedFiles(projectDir)
	if listErr != nil {
		generated = []string{}
	}

	return map[string]interface{}{
		"status":          "ok",
		"project_path":    projectDir,
		"flags":           artifactFlags,
		"generated_files": generated,
		"next_steps":      []string{"sync_dependencies"},
	}, nil
}

func (s *mcpServer) toolSyncDependencies(args map[string]interface{}) (map[string]interface{}, error) {
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project_path is outside workspace", "Provide a project_path within --workspace", err)
	}
	mode := getMode(args)

	if mode == "dry_run" {
		return map[string]interface{}{
			"status":        "dry_run",
			"project_path":  projectDir,
			"planned_files": []string{"go.mod", "go.sum"},
			"planned_steps": []string{
				"run go mod tidy",
			},
		}, nil
	}

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, toolError("dependency_sync_failed", "go mod tidy failed", "Fix module/import issues and retry sync_dependencies", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out))))
	}

	return map[string]interface{}{
		"status":       "ok",
		"project_path": projectDir,
		"output":       strings.TrimSpace(string(out)),
	}, nil
}

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

func contains(items []string, item string) bool {
	for _, it := range items {
		if it == item {
			return true
		}
	}
	return false
}

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

func predictCreateServiceFiles(projectPath string) []string {
	base := filepath.ToSlash(projectPath)
	return []string{
		base + "/.fabrica.yaml",
		base + "/apis.yaml",
		base + "/go.mod",
		base + "/README.md",
		base + "/cmd/server/main.go",
		base + "/cmd/server/runtime_helpers_generated.go",
		base + "/cmd/server/auth_helpers_generated.go",
		base + "/cmd/server/metrics_helpers_generated.go",
		base + "/internal/storage/storage.go",
		base + "/internal/storage/storage_generated.go",
	}
}

func predictAddResourceFiles(projectDir, resourceName, version string) ([]string, error) {
	var planned []string
	err := withWorkingDir(projectDir, func() error {
		apis, err := LoadAPIsConfig("")
		if err != nil {
			return err
		}
		group, err := apis.primaryGroup()
		if err != nil {
			return err
		}
		v := version
		if v == "" {
			v = group.StorageVersion
		}
		planned = []string{
			filepath.ToSlash(filepath.Join("apis", group.Name, v, strings.ToLower(resourceName)+"_types.go")),
			APIsConfigFileName,
		}
		return nil
	})
	return planned, err
}

func predictAddVersionFiles(projectDir, newVersion, fromVersion string) ([]string, error) {
	planned := make([]string, 0)
	err := withWorkingDir(projectDir, func() error {
		apis, err := LoadAPIsConfig("")
		if err != nil {
			return err
		}
		group, err := apis.primaryGroup()
		if err != nil {
			return err
		}
		source := fromVersion
		if source == "" {
			if len(group.Versions) == 0 {
				return fmt.Errorf("no existing versions found")
			}
			source = group.Versions[len(group.Versions)-1]
		}

		sourceDir := filepath.Join("apis", group.Name, source)
		entries, err := os.ReadDir(sourceDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_types.go") {
				continue
			}
			planned = append(planned, filepath.ToSlash(filepath.Join("apis", group.Name, newVersion, entry.Name())))
		}
		planned = append(planned, APIsConfigFileName)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(planned)
	return planned, nil
}

func predictGenerateImpact(projectDir string, flags []string) []string {
	impacts := make([]string, 0)
	add := func(path string) {
		for _, existing := range impacts {
			if existing == path {
				return
			}
		}
		impacts = append(impacts, path)
	}

	if len(flags) == 0 {
		add("cmd/server/*_generated.go")
		add("pkg/client/*.go")
		add("pkg/resources/register_generated.go")
		add("internal/storage/*")
		add("cmd/server/openapi_generated.go")
	} else {
		for _, flag := range flags {
			switch flag {
			case "--handlers":
				add("cmd/server/*_handlers_generated.go")
				add("cmd/server/routes_generated.go")
			case "--storage":
				add("internal/storage/*")
			case "--client":
				add("pkg/client/*.go")
			case "--openapi":
				add("cmd/server/openapi_generated.go")
			}
		}
		add("pkg/resources/register_generated.go")
	}

	if generated, err := listGeneratedFiles(projectDir); err == nil {
		for _, file := range generated {
			add(file)
		}
	}

	sort.Strings(impacts)
	return impacts
}

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

func runWithSilencedOutput(fn func() error) error {
	oldOut := os.Stdout
	oldErr := os.Stderr

	outR, outW, err := os.Pipe()
	if err != nil {
		return err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		return err
	}

	os.Stdout = outW
	os.Stderr = errW

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, outR)
		_, _ = io.Copy(io.Discard, errR)
		close(done)
	}()

	callErr := fn()

	_ = outW.Close()
	_ = errW.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	<-done
	_ = outR.Close()
	_ = errR.Close()

	return callErr
}

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

func (s *mcpServer) writeResult(id interface{}, result interface{}) error {
	resp := mcpResponse{JSONRPC: "2.0", ID: id, Result: result}
	return writeMCPMessage(s.out, resp)
}

func (s *mcpServer) writeError(id interface{}, code int, message string, data interface{}) error {
	resp := mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpError{Code: code, Message: message, Data: data}}
	return writeMCPMessage(s.out, resp)
}

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

func (r mcpRequest) idValue() interface{} {
	if len(r.ID) == 0 {
		return nil
	}
	var any interface{}
	if err := json.Unmarshal(r.ID, &any); err != nil {
		return nil
	}
	return any
}

func schemaObject(props map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
}

func schemaObjectWithRequired(props map[string]interface{}, required []string) map[string]interface{} {
	s := schemaObject(props)
	s["required"] = required
	return s
}

func modeField() map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"enum":        []string{"dry_run", "execute"},
		"description": "Operation mode",
	}
}

func getString(args map[string]interface{}, key, def string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func getBool(args map[string]interface{}, key string, def bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func getMode(args map[string]interface{}) string {
	mode := getString(args, "mode", "execute")
	if mode != "dry_run" {
		return "execute"
	}
	return mode
}

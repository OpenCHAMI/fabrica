// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	configpkg "github.com/openchami/fabrica/internal/config"
)

// Tool Implementations

// toolInspectProject returns information about a Fabrica project's configuration and state.
func (s *mcpServer) toolInspectProject(args map[string]interface{}) (map[string]interface{}, error) {
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project path is outside workspace", "Provide a project_path within --workspace", err)
	}

	var cfg *configpkg.FabricaConfig
	var apis *configpkg.APIsConfig
	var resources []string

	err = withWorkingDir(projectDir, func() error {
		var loadErr error
		cfg, loadErr = configpkg.LoadConfig("")
		if loadErr != nil {
			return toolError("missing_config", "failed to load .fabrica.yaml", "Run fabrica init in the project root or set project_path correctly", loadErr)
		}
		apis, loadErr = configpkg.LoadAPIsConfig("")
		if loadErr != nil {
			return toolError("missing_apis_config", "failed to load apis.yaml", "Ensure apis.yaml exists in project root", loadErr)
		}
		resources, loadErr = discoverResources(apis)
		return loadErr
	})
	if err != nil {
		return nil, err
	}

	group, err := apis.PrimaryGroup()
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
		"resource_files":         resourceFileMap(projectDir, group),
		"endpoints":              resourceEndpoints(resources),
		"generated_files":        mustListGeneratedFiles(projectDir),
		"docs_url":               "http://localhost:8080/docs",
		"openapi_url":            "http://localhost:8080/openapi.json",
		"server_command":         "go run ./cmd/server",
		"recommended_next_calls": recommendedCalls("inspect_project", projectDir),
	}, nil
}

// toolValidateProject checks project structure and configuration consistency.
func (s *mcpServer) toolValidateProject(args map[string]interface{}) (map[string]interface{}, error) {
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project path is outside workspace", "Provide a project_path within --workspace", err)
	}

	issues := make([]map[string]interface{}, 0)
	addIssue := func(severity, code, message string) {
		issues = append(issues, map[string]interface{}{"severity": severity, "code": code, "message": message})
	}

	// Check for required configuration files
	cfgPath := filepath.Join(projectDir, configpkg.ConfigFileName)
	if _, err := fileInfo(cfgPath); err != nil {
		addIssue("error", "missing_config", fmt.Sprintf("missing %s", configpkg.ConfigFileName))
	}
	apisPath := filepath.Join(projectDir, configpkg.APIsConfigFileName)
	if _, err := fileInfo(apisPath); err != nil {
		addIssue("error", "missing_apis_config", fmt.Sprintf("missing %s", configpkg.APIsConfigFileName))
	}

	// Perform detailed validation if core files exist
	if len(issues) == 0 {
		err = withWorkingDir(projectDir, func() error {
			cfg, loadErr := configpkg.LoadConfig("")
			if loadErr != nil {
				return loadErr
			}
			if validateErr := configpkg.ValidateConfig(cfg); validateErr != nil {
				addIssue("error", "invalid_config", validateErr.Error())
			}

			apis, loadErr := configpkg.LoadAPIsConfig("")
			if loadErr != nil {
				return loadErr
			}
			if validateErr := apis.Validate(); validateErr != nil {
				addIssue("error", "invalid_apis_config", validateErr.Error())
			}

			group, groupErr := apis.PrimaryGroup()
			if groupErr != nil {
				addIssue("error", "invalid_primary_group", groupErr.Error())
				return nil
			}

			// Validate version directories exist
			for _, v := range group.Versions {
				versionDir := filepath.Join("apis", group.Name, v)
				if _, statErr := fileInfo(versionDir); statErr != nil {
					addIssue("warning", "missing_version_dir", fmt.Sprintf("version directory not found: %s", versionDir))
				}
			}

			// Check resource list consistency
			storageResourceFiles, fileErr := listResourceTypeFiles(filepath.Join("apis", group.Name, group.StorageVersion))
			if fileErr == nil {
				storageResources := make(map[string]struct{}, len(storageResourceFiles))
				for _, name := range storageResourceFiles {
					storageResources[name] = struct{}{}
				}

				resourceNames := group.Resources.Names()
				for _, resource := range resourceNames {
					if _, ok := storageResources[resource]; !ok {
						addIssue("warning", "resource_list_drift", fmt.Sprintf("resource %s listed in apis.yaml but not found in storage version dir", resource))
					}
				}
				for _, resource := range storageResourceFiles {
					if !contains(resourceNames, resource) {
						addIssue("warning", "resource_list_drift", fmt.Sprintf("resource %s exists in storage version dir but is missing from apis.yaml resources", resource))
					}
				}
			}

			// Check for discovered resources
			resources, discErr := discoverResources(apis)
			if discErr != nil {
				addIssue("error", "resource_discovery_failed", discErr.Error())
			} else if len(resources) == 0 {
				addIssue("warning", "no_resources", "no resources found; run fabrica add resource <name>")
			}

			// Check if generated artifacts are stale
			routesPath := filepath.Join("cmd", "server", "routes_generated.go")
			routesInfo, statErr := fileInfo(routesPath)
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

// toolCreateService scaffolds a new Fabrica service project.
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
		withStorage:        true,
		withVersion:        true,
		storageType:        getString(args, "storage_type", "file"),
		dbDriver:           getString(args, "db", "sqlite"),
		validationMode:     getString(args, "validation_mode", "strict"),
		eventBusType:       getString(args, "events_bus", "memory"),
		apiGroup:           getString(args, "group", ""),
		storageVersion:     getString(args, "storage_version", "v1"),
		modulePath:         getString(args, "module", ""),
		description:        getString(args, "description", ""),
		withAuth:           getBool(args, "auth", false),
		withMetrics:        getBool(args, "metrics", false),
		withEvents:         getBool(args, "events", false),
		withReconcile:      getBool(args, "reconcile", false),
		reconcileWorkers:   getNumber(args, "reconcile_workers", 5),
		reconcileRequeueMs: getNumber(args, "reconcile_requeue", 5),
	}
	if versions := getStringArray(args, "versions", nil); len(versions) > 0 {
		opts.apiVersions = versions
	} else {
		opts.apiVersions = []string{"v1"}
	}
	if err := validateInitOptions(opts); err != nil {
		return nil, toolError("invalid_arguments", "invalid service options", "Adjust create_service arguments and retry", err)
	}

	if mode == "dry_run" {
		createdPath := filepath.Join(targetDir, projectName)
		if projectName == "." {
			createdPath = targetDir
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
			"recommended_next_calls": recommendedCalls("create_service", createdPath),
		}, nil
	}

	var output string
	err = withWorkingDir(targetDir, func() error {
		var runErr error
		output, runErr = runFabricaCLI(targetDir, initArgs(projectName, opts)...)
		return runErr
	})
	if err != nil {
		return nil, toolError("create_service_failed", "failed to create Fabrica service", "Check if target path exists and is writable. Output: "+truncateOutput(output), err)
	}

	createdPath := filepath.Join(targetDir, projectName)
	if projectName == "." {
		createdPath = targetDir
	}

	return map[string]interface{}{
		"status":                 "ok",
		"project_name":           projectName,
		"created_path":           createdPath,
		"next_steps":             []string{"add_resource", "generate_code", "sync_dependencies"},
		"recommended_next_calls": recommendedCalls("create_service", createdPath),
		"output":                 truncateOutput(output),
		"execution_mode":         mode,
	}, nil
}

// toolAddResource adds a new resource type to a project.
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
			planned = []string{filepath.ToSlash(filepath.Join("apis", "<group>", "<version>", strings.ToLower(resourceName)+"_types.go")), configpkg.APIsConfigFileName}
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
			"recommended_next_calls": recommendedCalls("add_resource", projectDir),
		}, nil
	}

	var output string
	err = withWorkingDir(projectDir, func() error {
		var runErr error
		output, runErr = runFabricaCLI(projectDir, addResourceArgs(resourceName, opts)...)
		return runErr
	})
	if err != nil {
		return nil, toolError("add_resource_failed", "failed to add resource", "Ensure project has valid .fabrica.yaml and apis.yaml. Output: "+truncateOutput(output), err)
	}

	resourceFile, _ := findResourceTypeFile(projectDir, resourceName, opts.version)
	return map[string]interface{}{
		"status":                 "ok",
		"resource_name":          resourceName,
		"project_path":           projectDir,
		"resource_file":          resourceFile,
		"next_steps":             []string{"generate_code", "sync_dependencies"},
		"recommended_next_calls": recommendedCalls("add_resource", projectDir),
		"output":                 truncateOutput(output),
	}, nil
}

// toolDefineResourceSchema updates a resource's Spec/Status field definitions.
func (s *mcpServer) toolDefineResourceSchema(args map[string]interface{}) (map[string]interface{}, error) {
	resourceName := strings.TrimSpace(getString(args, "resource_name", ""))
	if resourceName == "" {
		return nil, toolError("invalid_arguments", "resource_name is required", "Provide a non-empty resource_name", nil)
	}
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project_path is outside workspace", "Provide a project_path within --workspace", err)
	}
	mode := getMode(args)
	version := getString(args, "version", "")

	specFields, err := parseMCPFields(args["spec_fields"])
	if err != nil {
		return nil, toolError("invalid_arguments", "invalid spec_fields", "Provide spec_fields as an array of field objects", err)
	}
	statusFields, err := parseMCPFields(args["status_fields"])
	if err != nil {
		return nil, toolError("invalid_arguments", "invalid status_fields", "Provide status_fields as an array of field objects", err)
	}
	if len(specFields) == 0 && len(statusFields) == 0 {
		return nil, toolError("invalid_arguments", "at least one spec or status field is required", "Provide spec_fields, status_fields, or both", nil)
	}

	resourceFile, err := findResourceTypeFile(projectDir, resourceName, version)
	if err != nil {
		return nil, toolError("resource_not_found", "resource type file not found", "Run add_resource first or pass the correct version", err)
	}
	planned := []string{resourceFile}
	if mode == "dry_run" {
		return map[string]interface{}{
			"status":                 "dry_run",
			"project_path":           projectDir,
			"resource_name":          resourceName,
			"resource_file":          resourceFile,
			"planned_files":          planned,
			"planned_steps":          []string{"replace Spec/Status struct field declarations", "format resource type file"},
			"recommended_next_calls": recommendedCalls("define_resource_schema", projectDir),
		}, nil
	}

	if err := rewriteResourceSchema(resourceFile, resourceName, specFields, statusFields); err != nil {
		return nil, toolError("schema_update_failed", "failed to update resource schema", "Check field names/types and retry", err)
	}

	return map[string]interface{}{
		"status":                 "ok",
		"project_path":           projectDir,
		"resource_name":          resourceName,
		"resource_file":          resourceFile,
		"updated_files":          planned,
		"next_steps":             []string{"generate_code", "sync_dependencies", "build_project"},
		"recommended_next_calls": recommendedCalls("define_resource_schema", projectDir),
	}, nil
}

// toolAddVersion adds a new API version to a project.
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
			planned = []string{filepath.ToSlash(filepath.Join("apis", "<group>", newVersion, "*_types.go")), configpkg.APIsConfigFileName}
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
			"recommended_next_calls": recommendedCalls("add_version", projectDir),
		}, nil
	}

	var output string
	err = withWorkingDir(projectDir, func() error {
		var runErr error
		output, runErr = runFabricaCLI(projectDir, addVersionArgs(newVersion, opts)...)
		return runErr
	})
	if err != nil {
		return nil, toolError("add_version_failed", "failed to add API version", "Ensure project has valid apis.yaml and source version exists. Output: "+truncateOutput(output), err)
	}

	return map[string]interface{}{
		"status":                 "ok",
		"new_version":            newVersion,
		"project_path":           projectDir,
		"next_steps":             []string{"generate_code", "sync_dependencies"},
		"recommended_next_calls": recommendedCalls("add_version", projectDir),
		"output":                 truncateOutput(output),
	}, nil
}

// toolGenerateCode runs code generation for a project.
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
		willWrite, possible := predictGenerateImpact(projectDir, artifactFlags)
		return map[string]interface{}{
			"status":         "dry_run",
			"project_path":   projectDir,
			"flags":          artifactFlags,
			"planned_files":  willWrite,
			"possible_files": possible,
			"planned_steps": []string{
				"discover resources",
				"refresh registration",
				"generate server/client/storage/openapi artifacts",
			},
			"recommended_next_calls": recommendedCalls("generate_code", projectDir),
		}, nil
	}

	var output string
	err = withWorkingDir(projectDir, func() error {
		var runErr error
		output, runErr = runFabricaCLI(projectDir, generateArgs(artifactFlags)...)
		return runErr
	})
	if err != nil {
		return nil, toolError("generate_failed", "fabrica generate failed", "Inspect resource definitions and retry, or use debug=true. Output: "+truncateOutput(output), err)
	}

	generated, listErr := listGeneratedFiles(projectDir)
	if listErr != nil {
		generated = []string{}
	}

	return map[string]interface{}{
		"status":                 "ok",
		"project_path":           projectDir,
		"flags":                  artifactFlags,
		"generated_files":        generated,
		"next_steps":             []string{"sync_dependencies"},
		"recommended_next_calls": recommendedCalls("generate_code", projectDir),
		"output":                 truncateOutput(output),
	}, nil
}

// toolSyncDependencies runs go mod tidy for a project.
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
			"recommended_next_calls": recommendedCalls("sync_dependencies", projectDir),
		}, nil
	}

	out, err := execCommand(projectDir, "go", []string{"mod", "tidy"})
	if err != nil {
		return nil, toolError("dependency_sync_failed", "go mod tidy failed", "Fix module/import issues and retry sync_dependencies", fmt.Errorf("%v: %s", err, out))
	}

	return map[string]interface{}{
		"status":                 "ok",
		"project_path":           projectDir,
		"output":                 truncateOutput(out),
		"recommended_next_calls": recommendedCalls("sync_dependencies", projectDir),
	}, nil
}

// toolBuildProject runs go build for a project.
func (s *mcpServer) toolBuildProject(args map[string]interface{}) (map[string]interface{}, error) {
	return s.toolGoCommand(args, "build_project", "go build", "go", append([]string{"build"}, getStringArray(args, "packages", []string{"./..."})...))
}

// toolTestProject runs go test for a project.
func (s *mcpServer) toolTestProject(args map[string]interface{}) (map[string]interface{}, error) {
	return s.toolGoCommand(args, "test_project", "go test", "go", append([]string{"test"}, getStringArray(args, "packages", []string{"./..."})...))
}

// toolGoCommand is a generic tool for running Go commands (build, test, etc).
func (s *mcpServer) toolGoCommand(args map[string]interface{}, toolName, label, command string, commandArgs []string) (map[string]interface{}, error) {
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project_path is outside workspace", "Provide a project_path within --workspace", err)
	}
	mode := getMode(args)
	if mode == "dry_run" {
		return map[string]interface{}{
			"status":                 "dry_run",
			"project_path":           projectDir,
			"command":                strings.Join(append([]string{command}, commandArgs...), " "),
			"planned_steps":          []string{label},
			"recommended_next_calls": recommendedCalls(toolName, projectDir),
		}, nil
	}

	out, err := execCommand(projectDir, command, commandArgs)
	if err != nil {
		return nil, toolError(toolName+"_failed", label+" failed", "Review output, fix compile/test errors, and retry", fmt.Errorf("%w: %s", err, out))
	}

	return map[string]interface{}{
		"status":                 "ok",
		"project_path":           projectDir,
		"command":                strings.Join(append([]string{command}, commandArgs...), " "),
		"output":                 truncateOutput(out),
		"recommended_next_calls": recommendedCalls(toolName, projectDir),
	}, nil
}

// toolSmokeTestAPI performs health and OpenAPI checks on a running or started server.
func (s *mcpServer) toolSmokeTestAPI(args map[string]interface{}) (map[string]interface{}, error) {
	projectDir, err := s.resolveProjectPath(getString(args, "project_path", "."))
	if err != nil {
		return nil, toolError("workspace_violation", "project_path is outside workspace", "Provide a project_path within --workspace", err)
	}
	mode := getMode(args)
	baseURL := strings.TrimRight(getString(args, "base_url", "http://localhost:8080"), "/")
	startServer := getBool(args, "start_server", false)
	timeout := getNumber(args, "timeout_seconds", 20)
	serverArgs := getStringArray(args, "server_arguments", []string{"./cmd/server"})
	if len(serverArgs) == 0 {
		serverArgs = []string{"./cmd/server"}
	}

	if mode == "dry_run" {
		steps := []string{"GET " + baseURL + "/health", "GET " + baseURL + "/openapi.json"}
		if startServer {
			steps = append([]string{"go run " + strings.Join(serverArgs, " ")}, steps...)
		}
		return map[string]interface{}{
			"status":                 "dry_run",
			"project_path":           projectDir,
			"base_url":               baseURL,
			"planned_steps":          steps,
			"recommended_next_calls": recommendedCalls("smoke_test_api", projectDir),
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	var serverOutput bytes.Buffer
	var serverCmd *exec.Cmd
	if startServer {
		serverCmd = exec.CommandContext(ctx, "go", append([]string{"run"}, serverArgs...)...)
		serverCmd.Dir = projectDir
		serverCmd.Stdout = &serverOutput
		serverCmd.Stderr = &serverOutput
		if err := serverCmd.Start(); err != nil {
			return nil, toolError("smoke_start_failed", "failed to start API server", "Ensure generated server builds and retry", err)
		}
		defer func() {
			cancel()
			_ = serverCmd.Wait()
		}()
	}

	healthStatus, healthErr := waitForHTTP(ctx, baseURL+"/health")
	openAPIStatus, openAPIErr := waitForHTTP(ctx, baseURL+"/openapi.json")
	if healthErr != nil || openAPIErr != nil {
		return nil, toolError("smoke_test_failed", "API smoke test failed", "Start the server or fix generated runtime errors. Server output: "+truncateOutput(serverOutput.String()), fmt.Errorf("health=%v (%v), openapi=%v (%v)", healthStatus, healthErr, openAPIStatus, openAPIErr))
	}

	return map[string]interface{}{
		"status":       "ok",
		"project_path": projectDir,
		"base_url":     baseURL,
		"checks": map[string]interface{}{
			"health_status":  healthStatus,
			"openapi_status": openAPIStatus,
		},
		"server_output":          truncateOutput(serverOutput.String()),
		"recommended_next_calls": recommendedCalls("smoke_test_api", projectDir),
	}, nil
}

// toolDescribeWorkflow returns MCP tool call sequences for common workflows.
func (s *mcpServer) toolDescribeWorkflow(args map[string]interface{}) (map[string]interface{}, error) {
	goal := getString(args, "goal", "new_crud_api")
	return map[string]interface{}{
		"status":   "ok",
		"goal":     goal,
		"workflow": workflowForGoal(goal),
	}, nil
}

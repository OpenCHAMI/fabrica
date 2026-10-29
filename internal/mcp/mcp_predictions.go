// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	configpkg "github.com/openchami/fabrica/internal/config"
	"github.com/openchami/fabrica/internal/projectfeatures"
	"github.com/openchami/fabrica/pkg/codegen"
)

// File Impact Prediction

// predictCreateServiceFiles returns the files that will be created for a new service.
func predictCreateServiceFiles(projectPath string, opts *initOptions) []string {
	base := filepath.ToSlash(projectPath)
	files := []string{
		base + "/.fabrica.yaml", base + "/apis.yaml", base + "/go.mod", base + "/README.md",
		base + "/cmd/server/main.go", base + "/cmd/server/runtime_helpers_generated.go",
		base + "/cmd/server/features_generated.go", base + "/cmd/server/features_manifest_generated.json",
		base + "/internal/storage/storage.go",
		base + "/internal/middleware/conditional_middleware_generated.go",
		base + "/internal/middleware/versioning_middleware_generated.go",
	}
	if opts.withAuth {
		files = append(files, base+"/cmd/server/auth_helpers_generated.go")
	}
	if opts.withMetrics {
		files = append(files, base+"/cmd/server/metrics_generated.go")
	}
	if opts.withEvents {
		files = append(files, base+"/internal/middleware/event_bus_generated.go")
	}
	if opts.validationMode != "disabled" {
		files = append(files, base+"/internal/middleware/validation_middleware_generated.go")
	}
	sort.Strings(files)
	return files
}

// predictAddResourceFiles returns files that will be created/modified for a new resource.
func predictAddResourceFiles(projectDir, resourceName, version string) ([]string, error) {
	var planned []string
	err := withWorkingDir(projectDir, func() error {
		apis, err := configpkg.LoadAPIsConfig("")
		if err != nil {
			return err
		}
		group, err := apis.PrimaryGroup()
		if err != nil {
			return err
		}
		v := version
		if v == "" {
			v = group.StorageVersion
		}
		planned = []string{
			filepath.ToSlash(filepath.Join("apis", group.Name, v, strings.ToLower(resourceName)+"_types.go")),
			configpkg.APIsConfigFileName,
		}
		return nil
	})
	return planned, err
}

// predictAddVersionFiles returns files that will be created/modified for a new API version.
func predictAddVersionFiles(projectDir, newVersion, fromVersion string) ([]string, error) {
	planned := make([]string, 0)
	err := withWorkingDir(projectDir, func() error {
		apis, err := configpkg.LoadAPIsConfig("")
		if err != nil {
			return err
		}
		group, err := apis.PrimaryGroup()
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
		planned = append(planned, configpkg.APIsConfigFileName)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(planned)
	return planned, nil
}

// findResourceTypeFile locates a resource type file in the project.
func findResourceTypeFile(projectDir, resourceName, version string) (string, error) {
	var out string
	err := withWorkingDir(projectDir, func() error {
		apis, err := configpkg.LoadAPIsConfig("")
		if err != nil {
			return err
		}
		group, err := apis.PrimaryGroup()
		if err != nil {
			return err
		}
		v := version
		if v == "" {
			v = group.StorageVersion
		}
		path := filepath.Join(projectDir, "apis", group.Name, v, strings.ToLower(resourceName)+"_types.go")
		if _, err := os.Stat(path); err != nil {
			return err
		}
		out = path
		return nil
	})
	return out, err
}

// predictGenerateImpact returns files that will be written/overwritten by generation.
// willWrite contains files that generation will definitely modify.
// possibleFiles contains all generated files that exist in the project.
func predictGenerateImpact(projectDir string, flags []string) ([]string, []string) {
	willWrite := make([]string, 0)
	add := func(path string) {
		for _, existing := range willWrite {
			if existing == path {
				return
			}
		}
		willWrite = append(willWrite, path)
	}

	hasArtifacts := false
	for _, flag := range flags {
		switch flag {
		case "--handlers", "--storage", "--client", "--openapi":
			hasArtifacts = true
		}
	}
	if !hasArtifacts {
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
				add("cmd/server/models_generated.go")
				add("internal/middleware/*_generated.go")
			case "--storage":
				add("internal/storage/*")
			case "--client":
				add("pkg/client/*.go")
				add("cmd/client/main.go")
			case "--openapi":
				add("cmd/server/openapi_generated.go")
				add("cmd/server/models_generated.go")
			}
		}
		add("pkg/resources/register_generated.go")
	}

	possibleFiles := make([]string, 0)
	if generated, err := listGeneratedFiles(projectDir); err == nil {
		possibleFiles = append(possibleFiles, generated...)
	}

	sort.Strings(willWrite)
	sort.Strings(possibleFiles)
	return willWrite, possibleFiles
}

// previewFeatureGeneration shares the CLI's preflight and exact feature-file scope.
func previewFeatureGeneration(projectDir string, flags []string) (writes, removals []string, err error) {
	serverDir := filepath.Join(projectDir, "cmd/server")
	adopted, err := codegen.UsesFeatureLoader(serverDir)
	if err != nil || !adopted {
		return nil, nil, err
	}
	selected, server := false, false
	for _, flag := range flags {
		switch flag {
		case "--handlers", "--storage":
			selected = true
			server = true
		case "--client", "--openapi":
			selected = true
		}
	}
	cfg, err := configpkg.LoadConfig(projectDir)
	if err != nil {
		return nil, nil, err
	}
	if err := configpkg.ValidateConfig(cfg); err != nil {
		return nil, nil, err
	}
	_, statErr := os.Stat(filepath.Join(projectDir, "apis.yaml"))
	g := codegen.NewGenerator(serverDir, "main", cfg.Project.Module)
	g.SetFeatures(projectfeatures.FromConfig(cfg, statErr == nil))
	if err := g.CheckFeatureGeneration(!selected); err != nil {
		return nil, nil, err
	}
	if selected && !server {
		return nil, nil, nil
	}
	return g.FeatureImpact()
}

// Workflow Recommendations

// recommendedCalls returns suggested next MCP tool calls based on the current stage.
func recommendedCalls(stage, projectPath string) []map[string]interface{} {
	switch stage {
	case "create_service":
		return []map[string]interface{}{
			{"tool": "add_resource", "arguments": map[string]interface{}{"project_path": projectPath, "resource_name": "Device", "mode": "dry_run"}},
		}
	case "add_resource":
		return []map[string]interface{}{
			{"tool": "define_resource_schema", "arguments": map[string]interface{}{"project_path": projectPath, "resource_name": "<resource>", "mode": "dry_run"}},
		}
	case "define_resource_schema", "add_version":
		return []map[string]interface{}{
			{"tool": "generate_code", "arguments": map[string]interface{}{"project_path": projectPath, "mode": "dry_run"}},
		}
	case "generate_code":
		return []map[string]interface{}{
			{"tool": "sync_dependencies", "arguments": map[string]interface{}{"project_path": projectPath, "mode": "dry_run"}},
		}
	case "sync_dependencies":
		return []map[string]interface{}{
			{"tool": "build_project", "arguments": map[string]interface{}{"project_path": projectPath, "mode": "dry_run"}},
			{"tool": "test_project", "arguments": map[string]interface{}{"project_path": projectPath, "mode": "dry_run"}},
		}
	case "build_project", "test_project":
		return []map[string]interface{}{
			{"tool": "smoke_test_api", "arguments": map[string]interface{}{"project_path": projectPath, "mode": "dry_run", "start_server": true}},
		}
	default:
		return []map[string]interface{}{
			{"tool": "validate_project", "arguments": map[string]interface{}{"project_path": projectPath}},
		}
	}
}

// workflowForGoal returns a complete MCP tool call sequence for a named goal.
// Supports:
//   - "new_crud_api": Create a new CRUD API from scratch
//   - "add_resource": Add a resource to an existing project
//   - "verify_project": Run validation and test suite
func workflowForGoal(goal string) []map[string]interface{} {
	switch goal {
	case "add_resource":
		return []map[string]interface{}{
			{"tool": "inspect_project", "arguments": map[string]interface{}{"project_path": "."}},
			{"tool": "add_resource", "arguments": map[string]interface{}{"project_path": ".", "resource_name": "<Resource>", "mode": "dry_run"}},
			{"tool": "define_resource_schema", "arguments": map[string]interface{}{
				"project_path": ".", "resource_name": "<Resource>", "mode": "dry_run",
				"spec_fields":   []interface{}{map[string]interface{}{"name": "Description", "type": "string", "json_name": "description"}},
				"status_fields": []interface{}{map[string]interface{}{"name": "Ready", "type": "bool", "json_name": "ready"}},
			}},
			{"tool": "generate_code", "arguments": map[string]interface{}{"project_path": ".", "mode": "dry_run"}},
			{"tool": "sync_dependencies", "arguments": map[string]interface{}{"project_path": ".", "mode": "dry_run"}},
			{"tool": "build_project", "arguments": map[string]interface{}{"project_path": ".", "mode": "dry_run"}},
		}
	case "verify_project":
		return []map[string]interface{}{
			{"tool": "validate_project", "arguments": map[string]interface{}{"project_path": "."}},
			{"tool": "sync_dependencies", "arguments": map[string]interface{}{"project_path": ".", "mode": "dry_run"}},
			{"tool": "build_project", "arguments": map[string]interface{}{"project_path": ".", "mode": "dry_run"}},
			{"tool": "test_project", "arguments": map[string]interface{}{"project_path": ".", "mode": "dry_run"}},
			{"tool": "smoke_test_api", "arguments": map[string]interface{}{"project_path": ".", "mode": "dry_run", "start_server": true}},
		}
	default: // "new_crud_api"
		return []map[string]interface{}{
			{"tool": "create_service", "arguments": map[string]interface{}{"project_name": "<project>", "mode": "dry_run"}},
			{"tool": "add_resource", "arguments": map[string]interface{}{"project_path": "<project>", "resource_name": "<Resource>", "mode": "dry_run"}},
			{"tool": "define_resource_schema", "arguments": map[string]interface{}{
				"project_path": "<project>", "resource_name": "<Resource>", "mode": "dry_run",
				"spec_fields":   []interface{}{map[string]interface{}{"name": "Description", "type": "string", "json_name": "description"}},
				"status_fields": []interface{}{map[string]interface{}{"name": "Ready", "type": "bool", "json_name": "ready"}},
			}},
			{"tool": "generate_code", "arguments": map[string]interface{}{"project_path": "<project>", "mode": "dry_run"}},
			{"tool": "sync_dependencies", "arguments": map[string]interface{}{"project_path": "<project>", "mode": "dry_run"}},
			{"tool": "build_project", "arguments": map[string]interface{}{"project_path": "<project>", "mode": "dry_run"}},
			{"tool": "test_project", "arguments": map[string]interface{}{"project_path": "<project>", "mode": "dry_run"}},
		}
	}
}

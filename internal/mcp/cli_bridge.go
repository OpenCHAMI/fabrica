// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package mcp provides Fabrica's built-in MCP server and supporting helpers.
package mcp

import (
	"fmt"
	"os"
	"path/filepath"

	configpkg "github.com/openchami/fabrica/internal/config"
)

type initOptions struct {
	modulePath         string
	description        string
	withAuth           bool
	withStorage        bool
	withMetrics        bool
	withVersion        bool
	validationMode     string
	withEvents         bool
	eventBusType       string
	apiGroup           string
	storageVersion     string
	apiVersions        []string
	withReconcile      bool
	reconcileWorkers   int
	reconcileRequeueMs int
	storageType        string
	dbDriver           string
}

type addOptions struct {
	withValidation bool
	withStatus     bool
	withVersioning bool
	packageName    string
	version        string
	force          bool
}

type versionOptions struct {
	from  string
	force bool
}

func validateInitOptions(opts *initOptions) error {
	if opts == nil {
		return fmt.Errorf("init options are required")
	}
	if !opts.withStorage {
		return fmt.Errorf("storage is required for generated CRUD APIs; omit --storage=false and use --storage-type file, --storage-type ent, or --storage-type custom")
	}
	if opts.withEvents && opts.eventBusType != "memory" {
		return fmt.Errorf("unsupported events bus %q: only memory is implemented", opts.eventBusType)
	}
	if opts.withReconcile && !opts.withEvents {
		return fmt.Errorf("reconciliation requires events; add --events or remove --reconcile")
	}
	return nil
}

func discoverResources(apisConfig *configpkg.APIsConfig) ([]string, error) {
	if apisConfig == nil {
		return nil, nil
	}
	group, err := apisConfig.PrimaryGroup()
	if err != nil {
		return nil, err
	}
	versionDir := filepath.Join("apis", group.Name, group.StorageVersion)
	if _, err := os.Stat(versionDir); err == nil {
		resources, listErr := listResourceTypeFiles(versionDir)
		if listErr == nil && len(resources) > 0 {
			return resources, nil
		}
	}
	return append([]string(nil), group.Resources...), nil
}

func fabricaBinary() (string, error) {
	if override := os.Getenv("FABRICA_MCP_TEST_BINARY"); override != "" {
		return override, nil
	}
	return os.Executable()
}

func runFabricaCLI(workingDir string, args ...string) (string, error) {
	binary, err := fabricaBinary()
	if err != nil {
		return "", err
	}
	return execCommand(workingDir, binary, args)
}

func initArgs(projectName string, opts *initOptions) []string {
	args := []string{"init", projectName}
	if opts.modulePath != "" {
		args = append(args, "--module", opts.modulePath)
	}
	if opts.description != "" {
		args = append(args, "--description", opts.description)
	}
	if opts.withAuth {
		args = append(args, "--auth")
	}
	if opts.withMetrics {
		args = append(args, "--metrics")
	}
	if opts.validationMode != "" {
		args = append(args, "--validation-mode", opts.validationMode)
	}
	if opts.withEvents {
		args = append(args, "--events")
	}
	if opts.eventBusType != "" {
		args = append(args, "--events-bus", opts.eventBusType)
	}
	if opts.apiGroup != "" {
		args = append(args, "--group", opts.apiGroup)
	}
	if opts.storageVersion != "" {
		args = append(args, "--storage-version", opts.storageVersion)
	}
	if len(opts.apiVersions) > 0 {
		args = append(args, "--versions", joinCSV(opts.apiVersions))
	}
	if opts.withReconcile {
		args = append(args, "--reconcile")
	}
	if opts.reconcileWorkers > 0 {
		args = append(args, "--reconcile-workers", fmt.Sprintf("%d", opts.reconcileWorkers))
	}
	if opts.reconcileRequeueMs > 0 {
		args = append(args, "--reconcile-requeue", fmt.Sprintf("%d", opts.reconcileRequeueMs))
	}
	if opts.storageType != "" {
		args = append(args, "--storage-type", opts.storageType)
	}
	if opts.dbDriver != "" {
		args = append(args, "--db", opts.dbDriver)
	}
	return args
}

func addResourceArgs(resourceName string, opts *addOptions) []string {
	args := []string{"add", "resource", resourceName}
	if !opts.withValidation {
		args = append(args, "--with-validation=false")
	}
	if !opts.withStatus {
		args = append(args, "--with-status=false")
	}
	if opts.withVersioning {
		args = append(args, "--with-versioning")
	}
	if opts.packageName != "" {
		args = append(args, "--package", opts.packageName)
	}
	if opts.version != "" {
		args = append(args, "--version", opts.version)
	}
	if opts.force {
		args = append(args, "--force")
	}
	return args
}

func addVersionArgs(newVersion string, opts *versionOptions) []string {
	args := []string{"add", "version", newVersion}
	if opts.from != "" {
		args = append(args, "--from", opts.from)
	}
	if opts.force {
		args = append(args, "--force")
	}
	return args
}

func generateArgs(flags []string) []string {
	args := []string{"generate"}
	args = append(args, flags...)
	return args
}

func joinCSV(values []string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for _, value := range values[1:] {
		result += "," + value
	}
	return result
}

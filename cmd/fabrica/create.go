// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// newCreateCommand creates the 'fabrica create' command.
//
// 'create' is 'init' plus '--resources': it initializes a new Fabrica project
// with the same options as 'fabrica init' and then adds resources from a
// directory of hand-written struct files. Each .go file in that directory that
// declares one exported <Name>Spec struct is added (like 'fabrica add resource
// <Name>') and its declarations are merged, with comments preserved, into the
// generated apis/<group>/<version>/<name>_types.go file.
func newCreateCommand() *cobra.Command {
	opts := defaultInitOptions()
	var resourcesPath string

	cmd := &cobra.Command{
		Use:   "create [project-name]",
		Short: "Create a new Fabrica project and add resources from struct files",
		Long: `Create a new Fabrica project and populate it with resources in one step.

'create' accepts all the flags of 'fabrica init', plus '--resources <specs-dir>'.
It initializes the project and then, for each .go file in <specs-dir> that
declares one exported <Name>Spec struct, runs the equivalent of 'fabrica add
resource <Name>' and merges the hand-written declarations (preserving comments)
into the generated types file.

Which resources are added is decided at run time from the files present in the
directory.

After 'create', run 'fabrica generate', then 'go mod tidy'.

Example:
  fabrica create inventory-service \
    --module github.com/openchami/inventory-service \
    --group inventory-service.openchami.org \
    --storage-type ent --db sqlite \
    --resources ./structs`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if resourcesPath == "" {
				return fmt.Errorf("--resources <specs-dir> is required for 'create'; use 'fabrica init' to create a project without resources")
			}
			if opts.interactive {
				return fmt.Errorf("'create' does not support --interactive; use 'fabrica init -i' then 'fabrica add resources <specs-dir>'")
			}

			// Resolve the resources directory to an absolute path before any
			// directory change, so it is interpreted relative to the original
			// working directory (not --dir or the new project directory).
			absResources, err := filepath.Abs(resourcesPath)
			if err != nil {
				return fmt.Errorf("failed to resolve --resources path: %w", err)
			}

			// Discover and validate resource parts before creating the project,
			// so bad input fails fast without leaving a partial project.
			parts, err := discoverResourceParts(absResources)
			if err != nil {
				return fmt.Errorf("--resources: %w", err)
			}

			if opts.workDir != "" {
				if err := os.MkdirAll(opts.workDir, 0755); err != nil {
					return fmt.Errorf("failed to create working directory: %w", err)
				}
				if err := os.Chdir(opts.workDir); err != nil {
					return fmt.Errorf("failed to change directory to %s: %w", opts.workDir, err)
				}
			}

			projectName := "myproject"
			if len(args) > 0 {
				projectName = args[0]
			}

			// If a non-default database driver is specified, automatically use ent storage
			if opts.dbDriver == "postgres" || opts.dbDriver == "mysql" {
				opts.storageType = "ent"
			}

			// If group is specified but no versions, default to storage version
			if opts.apiGroup != "" && len(opts.apiVersions) == 0 {
				opts.apiVersions = []string{opts.storageVersion}
			}

			if err := runInit(projectName, opts); err != nil {
				return err
			}

			// runInit creates the project in a directory named projectName
			// (or the current directory when projectName is ".").
			if err := addResourcesToProject(projectName, parts); err != nil {
				return fmt.Errorf("failed to add resources from %s: %w", resourcesPath, err)
			}

			fmt.Println("\n✅ Resources added successfully!")
			fmt.Println("\nNext steps:")
			fmt.Println("  1. Run 'fabrica generate' to generate code")
			fmt.Println("  2. Run 'go mod tidy' to update dependencies")
			return nil
		},
	}

	registerInitFlags(cmd, opts)
	cmd.Flags().StringVar(&resourcesPath, "resources", "", "Directory of hand-written spec struct files (one <Name>Spec per file) to add and merge (required)")

	return cmd
}

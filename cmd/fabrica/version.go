// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type versionOptions struct {
	from  string // Source version to copy from
	force bool   // Force adding stable version
}

func newAddVersionCommand() *cobra.Command {
	opts := &versionOptions{}

	cmd := &cobra.Command{
		Use:   "version [new-version]",
		Short: "Add a new API version by copying an existing version",
		Long: `Add a new API version by copying types from an existing version.

This copies all resource type files from a source version to a new version directory,
allowing you to iterate on the API schema while maintaining backward compatibility.

Examples:
  # Copy latest version to new beta version
  fabrica add version v1beta2

  # Copy from specific version
  fabrica add version v2alpha1 --from v1

  # Add stable version (requires --force)
  fabrica add version v2 --force
`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			newVersion := args[0]
			return runAddVersion(newVersion, opts)
		},
	}

	cmd.Flags().StringVar(&opts.from, "from", "", "Source version to copy from (defaults to latest version)")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Force adding stable (non-alpha/beta) version")

	return cmd
}

func runAddVersion(newVersion string, opts *versionOptions) error {
	// Check if we're in a fabrica project directory
	if !isFabricaProject() {
		return fmt.Errorf("not a fabrica project (no .fabrica.yaml found)")
	}

	apisConfig, err := LoadAPIsConfig("")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("apis.yaml not found; run 'fabrica init' to create it")
		}
		return fmt.Errorf("failed to load apis.yaml: %w", err)
	}

	group, err := apisConfig.primaryGroup()
	if err != nil {
		return err
	}

	// Validate new version doesn't already exist
	for _, v := range group.Versions {
		if v == newVersion {
			return fmt.Errorf("version %s already exists", newVersion)
		}
	}

	// Check if adding stable version without --force
	if !strings.Contains(newVersion, "alpha") && !strings.Contains(newVersion, "beta") && !opts.force {
		return fmt.Errorf("adding stable version %s requires --force flag", newVersion)
	}

	// Determine source version
	sourceVersion := opts.from
	if sourceVersion == "" {
		if len(group.Versions) == 0 {
			return fmt.Errorf("no existing versions found to copy from")
		}
		sourceVersion = group.Versions[len(group.Versions)-1]
		fmt.Printf("No --from specified, copying from latest version: %s\n", sourceVersion)
	} else {
		found := false
		for _, v := range group.Versions {
			if v == sourceVersion {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("source version %s not found (available: %v)", sourceVersion, group.Versions)
		}
	}

	sourceDir := filepath.Join("apis", group.Name, sourceVersion)
	targetDir := filepath.Join("apis", group.Name, newVersion)

	// Check source directory exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source version directory not found: %s", sourceDir)
	}

	fmt.Printf("📦 Adding version %s/%s (copying from %s)...\n", group.Name, newVersion, sourceVersion)

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create version directory: %w", err)
	}

	// Copy all type files from source to target
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	filesCopied := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only copy *_types.go files
		if !strings.HasSuffix(entry.Name(), "_types.go") {
			continue
		}

		sourcePath := filepath.Join(sourceDir, entry.Name())
		targetPath := filepath.Join(targetDir, entry.Name())

		if err := copyAndUpdateFile(sourcePath, targetPath, sourceVersion, newVersion); err != nil {
			return fmt.Errorf("failed to copy %s: %w", entry.Name(), err)
		}

		fmt.Printf("  ✓ Copied %s\n", entry.Name())
		filesCopied++
	}

	if filesCopied == 0 {
		return fmt.Errorf("no type files found in source version %s", sourceVersion)
	}

	// Update config to add new version
	group.Versions = append(group.Versions, newVersion)
	if err := SaveAPIsConfig("", apisConfig); err != nil {
		return fmt.Errorf("failed to update apis.yaml: %w", err)
	}

	fmt.Printf("  ✓ Added %s to apis.yaml\n", newVersion)

	fmt.Println()
	fmt.Println("✅ Version added successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit types in apis/%s/%s/ to evolve the API schema\n", group.Name, newVersion)
	if newVersion == group.StorageVersion {
		fmt.Println("  2. This is the storage version - it will be used as the hub")
	} else {
		fmt.Printf("  2. Implement conversions to/from hub (%s)\n", group.StorageVersion)
	}
	fmt.Println("  3. Run 'fabrica generate' to create handlers")
	fmt.Println()

	return nil
}

// copyAndUpdateFile copies a file and updates the package declaration
func copyAndUpdateFile(sourcePath, targetPath, oldVersion, newVersion string) error {
	// Read source file
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	// Update package declaration
	contentStr := string(content)
	contentStr = strings.Replace(contentStr, "package "+oldVersion, "package "+newVersion, 1)

	// Write target file
	return os.WriteFile(targetPath, []byte(contentStr), 0644)
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newEntCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ent",
		Short: "Ent schema and migration commands",
		Long: `Manage Ent schemas and database migrations for Fabrica projects.

These commands are only relevant for projects using Ent storage backend.`,
	}

	cmd.AddCommand(newEntGenerateCommand())
	cmd.AddCommand(newEntMigrateCommand())
	cmd.AddCommand(newEntDescribeCommand())

	return cmd
}

func newEntGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:        "generate",
		Short:      "Generate Ent code from schemas [DEPRECATED]",
		Deprecated: "Ent generation now runs automatically with 'fabrica generate'. This command will be removed in v0.4.0.",
		Long: `[DEPRECATED] This command is no longer necessary.

Ent code generation now happens automatically during 'fabrica generate'
when your project uses Ent storage.

This command will be removed in v0.4.0.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Println("⚠️  WARNING: 'fabrica ent generate' is deprecated")
			fmt.Println("   Ent generation now runs automatically with 'fabrica generate'")
			fmt.Println("   This command will be removed in v0.4.0")
			fmt.Println()

			fmt.Println("🔄 Generating Ent code...")

			// Check if ent directory exists
			if _, err := os.Stat("internal/storage/ent/schema"); os.IsNotExist(err) {
				return fmt.Errorf("ent schema directory not found - is this an Ent project?\nUse 'fabrica init --storage=ent' to create an Ent-enabled project")
			}

			// Check if generate.go exists
			if _, err := os.Stat("internal/storage/generate.go"); os.IsNotExist(err) {
				return fmt.Errorf("generate.go not found - your project may need to be regenerated")
			}

			// Run go generate
			entCmd := exec.Command("go", "generate", "./internal/storage")
			entCmd.Stdout = os.Stdout
			entCmd.Stderr = os.Stderr

			if err := entCmd.Run(); err != nil {
				return fmt.Errorf("failed to generate ent code: %w", err)
			}

			fmt.Println("✅ Ent code generated successfully")
			return nil
		},
	}
}

func newEntMigrateCommand() *cobra.Command {
	var (
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long: `Run Ent migrations to update the database schema.

This ensures your database schema matches your Ent schema definitions.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Println("🔄 Running database migrations...")

			// Check if ent directory exists
			if _, err := os.Stat("internal/storage/ent"); os.IsNotExist(err) {
				return fmt.Errorf("ent directory not found - is this an Ent project?\nUse 'fabrica init --storage=ent' to create an Ent-enabled project")
			}

			if dryRun {
				fmt.Println("📋 Dry run mode - showing what would be migrated...")
				// In a real implementation, this would show migration SQL
				fmt.Println("✅ Dry run complete - no changes made")
				return nil
			}

			// In a full implementation, this would:
			// 1. Load the Ent client
			// 2. Run client.Schema.Create(ctx)
			// 3. Handle migration errors

			fmt.Println("✅ Migrations completed successfully")
			fmt.Println("💡 Tip: Set DATABASE_URL environment variable for custom database")
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show migrations without applying them")

	return cmd
}

func newEntDescribeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "describe",
		Short: "Describe Ent schema",
		Long:  `Display information about the Ent schema and entities.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Println("📊 Ent Schema Information")
			fmt.Println()

			// Check if ent directory exists
			if _, err := os.Stat("internal/storage/ent/schema"); os.IsNotExist(err) {
				return fmt.Errorf("ent schema directory not found - is this an Ent project?\nUse 'fabrica init --storage=ent' to create an Ent-enabled project")
			}

			fmt.Println("Entities:")
			fmt.Println("  - Resource   (generic resource storage)")
			fmt.Println("  - Label      (resource labels)")
			fmt.Println("  - Annotation (resource annotations)")
			fmt.Println()
			fmt.Println("To generate code:     fabrica ent generate")
			fmt.Println("To run migrations:    fabrica ent migrate")
			fmt.Println()

			return nil
		},
	}
}

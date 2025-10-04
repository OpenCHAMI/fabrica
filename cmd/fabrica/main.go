// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package main provides the Fabrica CLI tool for scaffolding, code generation,
// and interactive project setup with tiered complexity.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "fabrica",
		Short: "Fabrica - Resource-based REST API framework",
		Long: `Fabrica is a powerful Go framework for building resource-based REST APIs
with automatic code generation, multi-version schema support, and pluggable storage.

The CLI provides commands for:
  - Project initialization with tiered complexity (simple/standard/expert)
  - Resource scaffolding and code generation
  - Interactive wizards for guided setup
  - Example generation with progressive disclosure
  - Documentation generation`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	}

	// Add commands
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newAddCommand())
	rootCmd.AddCommand(newGenerateCommand())
	rootCmd.AddCommand(newExampleCommand())
	rootCmd.AddCommand(newDocsCommand())
	rootCmd.AddCommand(newEntCommand())
	rootCmd.AddCommand(newVersionCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Fabrica version %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built: %s\n", date)
		},
	}
}

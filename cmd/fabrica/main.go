// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package main provides the Fabrica CLI tool for scaffolding, code generation,
// and interactive project setup with tiered complexity.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/openchami/fabrica/internal/mcp"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func resolveVersionInfo(currentVersion, currentCommit, currentDate string, buildInfo *debug.BuildInfo) (string, string, string) {
	if buildInfo == nil {
		return currentVersion, currentCommit, currentDate
	}

	resolvedVersion := currentVersion
	resolvedCommit := currentCommit
	resolvedDate := currentDate

	if resolvedVersion == "dev" && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
		resolvedVersion = buildInfo.Main.Version
	}

	if resolvedCommit != "none" && resolvedDate != "unknown" {
		return resolvedVersion, resolvedCommit, resolvedDate
	}

	for _, setting := range buildInfo.Settings {
		if resolvedCommit == "none" && setting.Key == "vcs.revision" && setting.Value != "" {
			resolvedCommit = setting.Value
		}
		if resolvedDate == "unknown" && setting.Key == "vcs.time" && setting.Value != "" {
			resolvedDate = setting.Value
		}
	}

	return resolvedVersion, resolvedCommit, resolvedDate
}

func resolveBuildVersionInfo() {
	if version != "dev" && commit != "none" && date != "unknown" {
		return
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	version, commit, date = resolveVersionInfo(version, commit, date, buildInfo)
}

func main() {
	resolveBuildVersionInfo()
	mcp.Version = version

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
	rootCmd.AddCommand(newEntCommand())
	rootCmd.AddCommand(mcp.NewCommand())
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
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("Fabrica version %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built: %s\n", date)
		},
	}
}

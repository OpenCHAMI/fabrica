// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionString_ReleaseBuildNoVCS(t *testing.T) {
	version = "v0.4.4"
	commit = "none"
	date = "unknown"
	if got := versionString(); got != "v0.4.4" {
		t.Fatalf("expected clean version string, got %q", got)
	}
}

func TestVersionString_FullBuild(t *testing.T) {
	version = "v0.4.4"
	commit = "abc1234"
	date = "2026-05-07T12:00:00Z"
	want := "v0.4.4 (commit: abc1234, built: 2026-05-07T12:00:00Z)"
	if got := versionString(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveVersionInfo_UsesModuleVersionWhenUnset(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.4.4"},
	}

	resolvedVersion, resolvedCommit, resolvedDate := resolveVersionInfo("dev", "none", "unknown", bi)

	if resolvedVersion != "v0.4.4" {
		t.Fatalf("expected version v0.4.4, got %q", resolvedVersion)
	}
	if resolvedCommit != "none" {
		t.Fatalf("expected commit none when unavailable, got %q", resolvedCommit)
	}
	if resolvedDate != "unknown" {
		t.Fatalf("expected date unknown when unavailable, got %q", resolvedDate)
	}
}

func TestResolveVersionInfo_UsesVCSSettingsWhenUnset(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-05-07T12:00:00Z"},
		},
	}

	resolvedVersion, resolvedCommit, resolvedDate := resolveVersionInfo("dev", "none", "unknown", bi)

	if resolvedVersion != "dev" {
		t.Fatalf("expected version to remain dev for devel builds, got %q", resolvedVersion)
	}
	if resolvedCommit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", resolvedCommit)
	}
	if resolvedDate != "2026-05-07T12:00:00Z" {
		t.Fatalf("expected vcs time to be used, got %q", resolvedDate)
	}
}

func TestResolveVersionInfo_PreservesLdflagsValues(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.4.4"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "frombuildinfo"},
			{Key: "vcs.time", Value: "frombuildinfo"},
		},
	}

	resolvedVersion, resolvedCommit, resolvedDate := resolveVersionInfo("v9.9.9", "ldflagscommit", "ldflagsdate", bi)

	if resolvedVersion != "v9.9.9" {
		t.Fatalf("expected ldflags version to be preserved, got %q", resolvedVersion)
	}
	if resolvedCommit != "ldflagscommit" {
		t.Fatalf("expected ldflags commit to be preserved, got %q", resolvedCommit)
	}
	if resolvedDate != "ldflagsdate" {
		t.Fatalf("expected ldflags date to be preserved, got %q", resolvedDate)
	}
}

func TestNewRootCommand_SilencesFrameworkErrorOutput(t *testing.T) {
	// Given
	cmd := newRootCommand()

	// When
	silenceErrors := cmd.SilenceErrors
	silenceUsage := cmd.SilenceUsage

	// Then
	if !silenceErrors {
		t.Fatal("expected root command to silence Cobra error rendering")
	}
	if !silenceUsage {
		t.Fatal("expected root command to silence Cobra usage rendering on errors")
	}
}

func TestBuiltCLI_VersionCommandRuns(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	binaryPath := filepath.Join(t.TempDir(), "fabrica")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = wd
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, output)
	}

	versionCmd := exec.Command(binaryPath, "version")
	versionCmd.Dir = wd
	output, err = versionCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("built version command failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "Fabrica version") {
		t.Fatalf("expected version output, got %q", string(output))
	}
}

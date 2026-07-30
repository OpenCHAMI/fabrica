// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type schemaHelperCommand struct {
	command *exec.Cmd
	output  bytes.Buffer
	result  string
}

func TestManagedSchemaSubprocess_live_same_root_has_one_owner(t *testing.T) {
	// Given
	dir := t.TempDir()
	current := filepath.Join(dir, "schema")
	ready := filepath.Join(dir, "ready")
	release := filepath.Join(dir, "release")
	owner := startSchemaHelper(t, schemaHelperConfig{
		current: current, ready: ready, release: release, result: filepath.Join(dir, "owner.result"), identity: "Live",
	})
	waitForTestPath(t, ready)
	contender := startSchemaHelper(t, schemaHelperConfig{
		current: current, ready: ready, release: release, result: filepath.Join(dir, "contender.result"), identity: "Contender",
	})
	waitForTestPath(t, contender.result)

	// When
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitSchemaHelper(t, owner)
	waitSchemaHelper(t, contender)

	// Then
	assertSchemaHelperStatuses(t, []*schemaHelperCommand{owner, contender})
	assertCoherentSubprocessSchema(t, current)
	assertNoTransactionArtifacts(t, current)
}

func TestManagedSchemaSubprocess_separate_roots_generate_concurrently(t *testing.T) {
	// Given
	dir := t.TempDir()
	firstRoot := filepath.Join(dir, "first", "schema")
	secondRoot := filepath.Join(dir, "second", "schema")
	firstReady := filepath.Join(dir, "first.ready")
	secondReady := filepath.Join(dir, "second.ready")
	release := filepath.Join(dir, "release")
	first := startSchemaHelper(t, schemaHelperConfig{
		current: firstRoot, ready: firstReady, release: release,
		result: filepath.Join(dir, "first.result"), identity: "First",
	})
	second := startSchemaHelper(t, schemaHelperConfig{
		current: secondRoot, ready: secondReady, release: release,
		result: filepath.Join(dir, "second.result"), identity: "Second",
	})
	waitForTestPath(t, firstReady)
	waitForTestPath(t, secondReady)

	// When
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitSchemaHelper(t, first)
	waitSchemaHelper(t, second)

	// Then
	assertSchemaHelperOwner(t, first)
	assertSchemaHelperOwner(t, second)
	assertCoherentSubprocessSchema(t, firstRoot)
	assertCoherentSubprocessSchema(t, secondRoot)
	assertNoTransactionArtifacts(t, firstRoot)
	assertNoTransactionArtifacts(t, secondRoot)
}

type schemaHelperConfig struct {
	current  string
	start    string
	ready    string
	release  string
	result   string
	identity string
	mode     string
	boundary string
}

func startSchemaHelper(t *testing.T, config schemaHelperConfig) *schemaHelperCommand {
	t.Helper()
	helper := &schemaHelperCommand{result: config.result}
	helperTemp := t.TempDir()
	helper.command = exec.Command(os.Args[0], "-test.run=^TestManagedSchemaSubprocessHelper$")
	helper.command.Env = append(os.Environ(),
		schemaHelperEnv+"=1",
		"TMPDIR="+helperTemp,
		"TMP="+helperTemp,
		"TEMP="+helperTemp,
		"FABRICA_SCHEMA_ROOT="+config.current,
		"FABRICA_SCHEMA_START="+config.start,
		"FABRICA_SCHEMA_READY="+config.ready,
		"FABRICA_SCHEMA_RELEASE="+config.release,
		"FABRICA_SCHEMA_RESULT="+config.result,
		"FABRICA_SCHEMA_ID="+config.identity,
		"FABRICA_SCHEMA_MODE="+config.mode,
		"FABRICA_SCHEMA_CRASH_BOUNDARY="+config.boundary,
	)
	helper.command.Stdout = &helper.output
	helper.command.Stderr = &helper.output
	if err := helper.command.Start(); err != nil {
		t.Fatal(err)
	}
	return helper
}

func waitSchemaHelper(t *testing.T, helper *schemaHelperCommand) {
	t.Helper()
	if err := helper.command.Wait(); err != nil {
		t.Fatalf("helper process failed: %v\n%s", err, helper.output.String())
	}
}

func waitForTestPath(t *testing.T, path string) {
	t.Helper()
	if err := waitForSchemaHelperPath(path); err != nil {
		t.Fatal(err)
	}
}

func assertSchemaHelperStatuses(t *testing.T, helpers []*schemaHelperCommand) {
	t.Helper()
	owners := 0
	for _, helper := range helpers {
		content, err := os.ReadFile(helper.result)
		if err != nil {
			t.Fatal(err)
		}
		status := string(content)
		switch status {
		case "owner":
			owners++
		case "busy":
		default:
			t.Errorf("unexpected helper status %q", status)
		}
	}
	if owners != 1 {
		t.Fatalf("owners = %d, want exactly one", owners)
	}
}

func assertSchemaHelperOwner(t *testing.T, helper *schemaHelperCommand) {
	t.Helper()
	content, err := os.ReadFile(helper.result)
	if err != nil || string(content) != "owner" {
		t.Fatalf("helper result = %q, %v", content, err)
	}
}

func assertCoherentSubprocessSchema(t *testing.T, current string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(current, "resource.go"))
	if err != nil || !isFabricaManagedSchema(content) || strings.Count(string(content), "type Owner") != 1 {
		t.Fatalf("subprocess schema is incoherent = %q, %v", content, err)
	}
}

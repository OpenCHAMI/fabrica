// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configpkg "github.com/openchami/fabrica/internal/config"
	"github.com/openchami/fabrica/internal/projectfeatures"
)

func TestFeatureRecoveryCheckoutCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("generated service integration")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	binary := filepath.Join(workspace, "fabrica")
	run := func(dir string, expectedSuccess bool, environment []string, executable string, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), environment...)
		output, err := cmd.CombinedOutput()
		if (err == nil) != expectedSuccess {
			t.Fatalf("%s %v: %v\n%s", executable, args, err, output)
		}
		return output
	}
	run(repo, true, nil, "go", "build", "-o", binary, "./cmd/fabrica")
	run(workspace, true, nil, binary, "init", "recovered", "--module", "example.com/recovered", "--storage-type", "file")
	dir := filepath.Join(workspace, "recovered")
	ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range []string{".fabrica-features.lock", ".fabrica-features-transaction.json", "*.fabrica-tmp"} {
		if !strings.Contains(string(ignore), pattern) {
			t.Fatalf("generated .gitignore omits %s", pattern)
		}
	}

	run(dir, true, nil, "go", "mod", "edit", "-replace", "github.com/openchami/fabrica="+repo)
	run(dir, true, nil, binary, "add", "resource", "Node")
	run(dir, true, nil, binary, "generate", "--fabrica-source", repo)
	server := filepath.Join(dir, "cmd/server")
	mainPath := filepath.Join(server, "main.go")
	main, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := configpkg.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Features.Metrics.Enabled = true
	if err := configpkg.SaveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	featureConfig, err := json.Marshal(projectfeatures.FromConfig(cfg, true))
	if err != nil {
		t.Fatal(err)
	}
	// Kill a real generator subprocess after one live feature file has changed.
	output := run(repo, false, []string{
		"FABRICA_TRANSACTION_TEST_SERVER=" + server,
		"FABRICA_TRANSACTION_TEST_MODULE=" + cfg.Project.Module,
		"FABRICA_TRANSACTION_TEST_CONFIG=" + string(featureConfig),
		"FABRICA_TRANSACTION_TEST_CRASH=after-rename:features_generated.go",
	}, "go", "test", "./pkg/codegen", "-run", "^TestFeatureTransactionCrashChild$", "-count=1")
	if !strings.Contains(string(output), "exit status 91") {
		t.Fatalf("helper did not interrupt generation: %s", output)
	}
	journal := filepath.Join(server, ".fabrica-features-transaction.json")
	recovery, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	registration := filepath.Join(dir, "pkg/resources/register_generated.go")
	before, err := os.ReadFile(registration)
	if err != nil {
		t.Fatal(err)
	}
	output = run(dir, false, nil, binary, "generate", "--handlers", "--fabrica-source", repo)
	if !strings.Contains(string(output), "recovery pending") {
		t.Fatalf("selective generation: %s", output)
	}
	after, err := os.ReadFile(registration)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("selective generation modified registration: %v", err)
	}
	after, err = os.ReadFile(journal)
	if err != nil || !bytes.Equal(recovery, after) {
		t.Fatalf("selective generation modified recovery evidence: %v", err)
	}
	// Recover the captured metrics-on transaction, then honor new metrics-off YAML.
	cfg.Features.Metrics.Enabled = false
	if err := configpkg.SaveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	run(dir, true, nil, binary, "generate", "--fabrica-source", repo)
	if _, err := os.Stat(journal); !os.IsNotExist(err) {
		t.Fatalf("journal remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(server, "metrics_generated.go")); !os.IsNotExist(err) {
		t.Fatalf("new selection not applied: %v", err)
	}
	after, err = os.ReadFile(mainPath)
	if err != nil || !bytes.Equal(main, after) {
		t.Fatalf("recovery changed user main: %v", err)
	}
	run(dir, true, nil, "go", "mod", "tidy")
	run(dir, true, nil, "go", "test", "./...")
	run(dir, true, nil, "go", "build", "./cmd/server")
	run(dir, true, nil, binary, "generate", "--fabrica-source", repo)
}

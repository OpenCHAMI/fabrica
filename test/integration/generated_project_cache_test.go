// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const generatedProjectHarnessPrefix = "fabrica-generated-project-harness-"

var (
	generatedProjectCacheRoot string
	generatedCLIOnce          sync.Once
	generatedCLIResult        commandResult
)

func TestMain(m *testing.M) {
	owner, err := currentGeneratedProjectOwner()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve generated-project owner: %v\n", err)
		os.Exit(1)
	}
	if err := scavengeStaleGeneratedProjectCaches(os.TempDir(), time.Now(), owner); err != nil {
		fmt.Fprintf(os.Stderr, "scavenge stale generated-project caches: %v\n", err)
		os.Exit(1)
	}
	root, err := os.MkdirTemp("", generatedProjectHarnessPrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create generated-project cache: %v\n", err)
		os.Exit(1)
	}
	if err := writeGeneratedProjectOwner(root, owner); err != nil {
		_ = os.RemoveAll(root)
		fmt.Fprintf(os.Stderr, "mark generated-project cache ownership: %v\n", err)
		os.Exit(1)
	}
	generatedProjectCacheRoot = root
	code := m.Run()
	if err := os.RemoveAll(root); err != nil {
		fmt.Fprintf(os.Stderr, "remove generated-project cache: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func generatedFabricaBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	binary := filepath.Join(generatedProjectCacheRoot, "tools", "fabrica")
	generatedCLIOnce.Do(func() {
		if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
			generatedCLIResult = commandResult{stage: "build-fabrica-cli", err: err}
			return
		}
		ctx, cancel := context.WithTimeout(t.Context(), generatedProjectCommandTimeout)
		defer cancel()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/fabrica")
		cmd.Dir = repoRoot
		cmd.Env = generatedProjectEnvironment()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if ctx.Err() != nil {
			err = fmt.Errorf("build-fabrica-cli timed out after %s: %w", generatedProjectCommandTimeout, ctx.Err())
		}
		generatedCLIResult = commandResult{stage: "build-fabrica-cli", stdout: stdout.String(), stderr: stderr.String(), err: err}
	})
	if generatedCLIResult.err != nil {
		t.Fatalf("%s", generatedCLIResult.failureMessage())
	}
	return binary
}

func generatedProjectEnvironment() []string {
	return append(
		os.Environ(),
		"GOWORK=off",
		"GOCACHE="+filepath.Join(generatedProjectCacheRoot, "build"),
	)
}

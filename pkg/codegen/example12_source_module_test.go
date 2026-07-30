// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExample12_source_and_generated_workflows_pass(t *testing.T) {
	// Given
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", filepath.Join(repoRoot, "examples", "12-storage-annotations", "verify-example.sh"))
	cmd.Dir = repoRoot

	// When
	output, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Example 12 verification failed: %v\n%s", err, output)
	}
	for _, receipt := range []string{
		"Example 12 source module preflight passed",
		"Example 12 API verification passed",
		"Example 12 generation/build regression passed",
	} {
		if !strings.Contains(string(output), receipt) {
			t.Errorf("verification output missing %q\n%s", receipt, output)
		}
	}
}

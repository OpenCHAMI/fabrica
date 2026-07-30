// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedTransactionHelper_baseline(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.run(t, "baseline-ent-codegen", project.root, "go", "generate", "./internal/storage"); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	path := filepath.Join(project.root, "internal", "storage", "generated_transaction_baseline_test.go")
	if err := os.WriteFile(path, []byte(generatedTransactionBaselineTest), 0o644); err != nil {
		t.Fatalf("write transaction baseline fixture: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	// When
	result := project.run(t, "generated-transaction-baseline", project.root, "go", "test", "-count=1", "-v", "-run", "TestGeneratedTransactionHelper", "./internal/storage")

	// Then
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if !strings.Contains(result.stdout, "transaction commit and rollback baseline") {
		t.Fatalf("missing transaction baseline receipt\n%s", result.stdout)
	}
}

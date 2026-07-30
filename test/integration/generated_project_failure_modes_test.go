// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeneratedProjectMatrix_rejects_unsupported_field_before_ent_codegen(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, generatedUnsupportedFieldSource)

	// When
	result := project.generate(t)

	// Then
	if result.err == nil {
		t.Fatalf("stage %q accepted unsupported map field", result.stage)
	}
	combined := result.stdout + result.stderr
	for _, contextPart := range []string{"token_types.go", "TokenSpec", "Payload", "not supported"} {
		if !strings.Contains(combined, contextPart) {
			t.Errorf("unsupported-field failure missing %q context\n%s", contextPart, result.failureMessage())
		}
	}
	schemaDir := filepath.Join(project.root, "internal", "storage", "ent", "schema")
	if _, err := os.Stat(schemaDir); !os.IsNotExist(err) {
		t.Errorf("unsupported field reached Ent generation; schema directory stat error: %v", err)
	}
}

func TestGeneratedProjectCommand_reports_bounded_timeout(t *testing.T) {
	// Given
	project := generatedProject{root: t.TempDir()}
	const timeout = 25 * time.Millisecond

	// When
	started := time.Now()
	result := project.runWithTimeout(t, timeout, "hung-command-probe", project.root, "sh", "-c", "exec sleep 5")
	elapsed := time.Since(started)

	// Then
	if result.err == nil || !strings.Contains(result.err.Error(), "timed out") {
		t.Fatalf("hung command result = %v, want explicit timeout", result.err)
	}
	if elapsed >= time.Second {
		t.Fatalf("hung command cleanup took %s, want less than 1s", elapsed)
	}
}

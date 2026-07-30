// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedDedicatedAdapter_roundtrips_complete_envelope_through_sqlite_reload(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, dedicatedEnvelopeTokenSource)
	generateResult := project.generate(t)
	if generateResult.err != nil {
		t.Fatalf("%s", generateResult.failureMessage())
	}
	preCodegenTidy := project.tidy(t)
	if preCodegenTidy.err != nil {
		t.Fatalf("%s", preCodegenTidy.failureMessage())
	}
	generateEntResult := project.run(t, "ent-codegen", project.root, "go", "generate", "./internal/storage")
	if generateEntResult.err != nil {
		t.Fatalf("%s", generateEntResult.failureMessage())
	}
	testPath := filepath.Join(project.root, "internal", "storage", "dedicated_envelope_runtime_test.go")
	if err := os.WriteFile(testPath, []byte(dedicatedEnvelopeRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated runtime test: %v", err)
	}
	tidyResult := project.tidy(t)
	if tidyResult.err != nil {
		t.Fatalf("%s", tidyResult.failureMessage())
	}

	// When
	runtimeResult := project.run(
		t,
		"dedicated-envelope-sqlite-runtime",
		project.root,
		"go",
		"test",
		"-count=1",
		"-v",
		"-run",
		"TestDedicatedEnvelope",
		"./internal/storage",
	)

	// Then
	if runtimeResult.err != nil {
		t.Fatalf("%s", runtimeResult.failureMessage())
	}
	for _, receipt := range []string{"standard field mapping", "reload round-trip", "nil and empty maps", "corrupt status"} {
		if !strings.Contains(runtimeResult.stdout, receipt) {
			t.Errorf("runtime output missing %q receipt\n%s", receipt, runtimeResult.stdout)
		}
	}
}

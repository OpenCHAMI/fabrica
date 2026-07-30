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

func TestDedicatedSecurity_generated_adapter_runtime(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, dedicatedSecurityTokenSource)
	generateResult := project.generate(t)
	if generateResult.err != nil {
		t.Fatalf("%s", generateResult.failureMessage())
	}
	if tidyResult := project.tidy(t); tidyResult.err != nil {
		t.Fatalf("%s", tidyResult.failureMessage())
	}
	entResult := project.run(t, "ent-codegen", project.root, "go", "generate", "./internal/storage")
	if entResult.err != nil {
		t.Fatalf("%s", entResult.failureMessage())
	}
	handlerResult := project.run(
		t,
		"fabrica-generate-handlers",
		project.root,
		project.fabricaBin,
		"generate",
		"--handlers",
		"--force",
		"--debug",
		"--fabrica-source",
		project.repoRoot,
	)
	if handlerResult.err != nil {
		t.Fatalf("%s", handlerResult.failureMessage())
	}
	testPath := filepath.Join(project.root, "internal", "storage", "dedicated_security_runtime_test.go")
	if err := os.WriteFile(testPath, []byte(dedicatedSecurityRuntimeTest+dedicatedSensitiveMatrixRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated security runtime test: %v", err)
	}
	handlerTestPath := filepath.Join(project.root, "cmd", "server", "dedicated_security_handler_runtime_test.go")
	if err := os.WriteFile(handlerTestPath, []byte(dedicatedSecurityHandlerRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated security handler runtime test: %v", err)
	}
	bodyTestPath := filepath.Join(project.root, "cmd", "server", "request_body_handler_runtime_test.go")
	if err := os.WriteFile(bodyTestPath, []byte(requestBodyHandlerRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated request body handler runtime test: %v", err)
	}
	routerBodyTestPath := filepath.Join(project.root, "cmd", "server", "request_body_router_runtime_test.go")
	if err := os.WriteFile(routerBodyTestPath, []byte(requestBodyRouterRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated request body router runtime test: %v", err)
	}
	routerConflictTestPath := filepath.Join(project.root, "cmd", "server", "conflict_router_runtime_test.go")
	if err := os.WriteFile(routerConflictTestPath, []byte(conflictRouterRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated conflict router runtime test: %v", err)
	}
	if tidyResult := project.tidy(t); tidyResult.err != nil {
		t.Fatalf("%s", tidyResult.failureMessage())
	}

	// When
	runtimeResult := project.run(
		t,
		"dedicated-security-sqlite-runtime",
		project.root,
		"go",
		"test",
		"-count=1",
		"-v",
		"-run",
		"TestDedicatedSecurity",
		"./internal/storage",
	)

	// Then
	if runtimeResult.err != nil {
		t.Fatalf("%s", runtimeResult.failureMessage())
	}
	handlerRuntimeResult := project.run(
		t,
		"dedicated-security-handler-sqlite-runtime",
		project.root,
		"go",
		"test",
		"-count=1",
		"-v",
		"-run",
		"TestDedicatedSecurity",
		"./cmd/server",
	)
	if handlerRuntimeResult.err != nil {
		t.Fatalf("%s", handlerRuntimeResult.failureMessage())
	}
	runtimeOutput := runtimeResult.stdout + handlerRuntimeResult.stdout
	t.Logf("generated security runtime output:\n%s", runtimeResult.stdout)
	for _, receipt := range []string{
		"required bcrypt create",
		"optional bcrypt omitted",
		"mutable bcrypt update",
		"immutable bcrypt skipped",
		"sensitive fields redacted",
		"redacted save preserves hidden fields",
		"explicit hidden replacements",
		"status-only persistence",
		"status conversion failure no write",
		"PATCH without credentials",
		"status endpoint update",
		"create persisted redacted response",
		"update persisted redacted response",
		"patch persisted redacted response",
		"PATCH duplicate conflict",
		"all body endpoints bounded",
		"exact body limit accepted",
		"malformed under limit unchanged",
		"live router body limits",
		"inventory-scale default accepted",
		"live router trailing body contracts",
		"live router conflicts",
		"sensitive type matrix",
		"bcrypt create failure no write",
		"bcrypt update failure no write",
	} {
		if !strings.Contains(runtimeOutput, receipt) {
			t.Errorf("runtime output missing %q receipt\n%s", receipt, runtimeOutput)
		}
	}
}

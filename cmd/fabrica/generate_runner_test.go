// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestGenerateRunnerCode_UsesStableAuthSetter(t *testing.T) {
	runnerCode := generateRunnerCode(
		"/tmp/project",
		"github.com/example/project",
		"cmd/server",
		"main",
		true,
		true,
		true,
		false,
		false,
		"file",
	)

	if !strings.Contains(runnerCode, "setAuthEnabledCompat(gen, authEnabled)") {
		t.Fatalf("runner code should configure auth via compatibility helper")
	}

	if strings.Contains(runnerCode, "gen.Config.WithAuth = true") {
		t.Fatalf("runner code should not write WithAuth directly")
	}

	if strings.Contains(runnerCode, "gen.Config.SecurityAuthNEnabled = true") {
		t.Fatalf("runner code should not write SecurityAuthNEnabled directly")
	}
}

func TestGenerateRunnerCode_SetsAuthForFalseAndTrue(t *testing.T) {
	runnerCode := generateRunnerCode(
		"/tmp/project",
		"github.com/example/project",
		"cmd/server",
		"main",
		true,
		false,
		false,
		false,
		false,
		"file",
	)

	if !strings.Contains(runnerCode, "setAuthEnabledCompat(gen, authEnabled)") {
		t.Fatalf("runner code must always pass through configured auth boolean")
	}
}

func TestGenerateRunnerCode_StorageOnlyDoesNotRegenerateRoutesOrModels(t *testing.T) {
	runnerCode := generateRunnerCode(
		"/tmp/project",
		"github.com/example/project",
		"cmd/server",
		"main",
		false,
		true,
		false,
		false,
		false,
		"file",
	)

	for _, unexpected := range []string{"GenerateRoutes", "GenerateModels", "GenerateOpenAPI"} {
		if strings.Contains(runnerCode, unexpected) {
			t.Fatalf("storage-only runner should not include %s", unexpected)
		}
	}
	if !strings.Contains(runnerCode, "GenerateStorage") {
		t.Fatalf("storage-only runner should generate storage")
	}
}

func TestGenerateRunnerCode_CustomStorageDoesNotGeneratePersistence(t *testing.T) {
	runnerCode := generateRunnerCode(
		"/tmp/project",
		"github.com/example/project",
		"cmd/server",
		"main",
		true,
		true,
		true,
		false,
		false,
		"custom",
	)

	for _, unexpected := range []string{
		"GenerateEntSchemas",
		"GenerateEntAdapter",
		"GenerateEntHelpers",
		"GenerateStorage",
		"GenerateExportCommand",
		"GenerateImportCommand",
	} {
		if strings.Contains(runnerCode, unexpected) {
			t.Fatalf("custom storage runner should not include %s", unexpected)
		}
	}
	for _, expected := range []string{"GenerateHandlers", "GenerateRoutes", "GenerateModels", "GenerateOpenAPI"} {
		if !strings.Contains(runnerCode, expected) {
			t.Fatalf("custom storage runner should still include %s", expected)
		}
	}
}

func TestGenerateRunnerCode_OpenAPIOnlyRegeneratesModelDependency(t *testing.T) {
	runnerCode := generateRunnerCode(
		"/tmp/project",
		"github.com/example/project",
		"cmd/server",
		"main",
		false,
		false,
		true,
		false,
		false,
		"file",
	)

	if !strings.Contains(runnerCode, "GenerateOpenAPI") {
		t.Fatalf("openapi runner should generate OpenAPI")
	}
	if !strings.Contains(runnerCode, "GenerateModels") {
		t.Fatalf("openapi runner should regenerate request model dependency")
	}
	if strings.Contains(runnerCode, "GenerateRoutes") || strings.Contains(runnerCode, "GenerateStorage") {
		t.Fatalf("openapi-only runner should not generate routes or storage")
	}
}

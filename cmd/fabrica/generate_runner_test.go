// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestGenerateRunnerCode_UsesStableAuthSetter(t *testing.T) {
	runnerCode := generateRunnerCode(
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

	if !strings.Contains(runnerCode, "gen.SetAuthEnabled(config.Features.Security.AuthN.Enabled)") {
		t.Fatalf("runner code should configure auth via SetAuthEnabled")
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

	if !strings.Contains(runnerCode, "gen.SetAuthEnabled(config.Features.Security.AuthN.Enabled)") {
		t.Fatalf("runner code must always pass through configured auth boolean")
	}
}

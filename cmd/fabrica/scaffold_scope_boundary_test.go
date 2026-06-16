// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_InitMain_OrchestrationOnlyBoundaries(t *testing.T) {
	mainTmpl := mustReadFile(t, "pkg/codegen/templates/init/main.go.tmpl")

	requiredOrchestrationCalls := []string{
		"initializeStorage(config)",
		"initializeEventingAndReconciliation(config)",
		"initializeAuthMiddleware(config)",
		"initializeMetricsServer(config)",
		"logStartupConfiguration(config, addr)",
	}

	for _, marker := range requiredOrchestrationCalls {
		if !strings.Contains(mainTmpl, marker) {
			t.Fatalf("init/main.go.tmpl must orchestrate via %q", marker)
		}
	}

	forbiddenFeatureImplementationMarkers := []string{
		"storage.InitFileBackend(config.DataDir)",
		"ent.Open(\"{{.DBDriver}}\"",
		"events.NewInMemoryEventBus(1000, 10)",
		"tokensmithauthn.Middleware(tokensmithauthn.Options{",
		"tokensmithauthz.NewMiddleware(",
		"func logAuthZDecision(_ context.Context, rec tokensmithauthz.DecisionRecord)",
		"func startMetricsServer(",
	}

	for _, marker := range forbiddenFeatureImplementationMarkers {
		if strings.Contains(mainTmpl, marker) {
			t.Fatalf("init/main.go.tmpl should not contain feature implementation marker %q", marker)
		}
	}
}

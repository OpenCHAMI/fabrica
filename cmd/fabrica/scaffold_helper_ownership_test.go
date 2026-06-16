// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_ScaffoldHelperOwnershipMarkers(t *testing.T) {
	runtimeTmpl := mustReadFile(t, "pkg/codegen/templates/init/runtime_helpers.go.tmpl")
	authTmpl := mustReadFile(t, "pkg/codegen/templates/init/auth_helpers.go.tmpl")
	metricsTmpl := mustReadFile(t, "pkg/codegen/templates/init/metrics_helpers.go.tmpl")

	for _, marker := range []string{
		"func initializeStorage(config *Config)",
		"storage.InitFileBackend(config.DataDir)",
		"events.NewInMemoryEventBus(1000, 10)",
		"func logStartupConfiguration(config *Config, addr string)",
	} {
		if !strings.Contains(runtimeTmpl, marker) {
			t.Fatalf("runtime helper template missing marker %q", marker)
		}
	}

	for _, marker := range []string{
		"func initializeAuthMiddleware(config *Config)",
		"tokensmithauthn.Middleware(tokensmithauthn.Options{",
		"tokensmithauthz.NewMiddleware(",
		"func logAuthZDecision(_ context.Context, rec tokensmithauthz.DecisionRecord)",
	} {
		if !strings.Contains(authTmpl, marker) {
			t.Fatalf("auth helper template missing marker %q", marker)
		}
	}

	for _, marker := range []string{
		"func initializeMetricsServer(config *Config)",
		"func startMetricsServer(config *Config)",
		"func metricsHandler(w http.ResponseWriter, r *http.Request)",
	} {
		if !strings.Contains(metricsTmpl, marker) {
			t.Fatalf("metrics helper template missing marker %q", marker)
		}
	}
}

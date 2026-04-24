// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitScaffold_EmitsHelperBoundaries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "scope-boundary-smoke")
	opts := &initOptions{
		modulePath:         "github.com/example/scope-boundary-smoke",
		description:        "scope boundary smoke",
		withAuth:           true,
		withStorage:        true,
		withMetrics:        true,
		withVersion:        true,
		validationMode:     "strict",
		withEvents:         true,
		eventBusType:       "memory",
		apiGroup:           "example.fabrica.dev",
		storageVersion:     "v1",
		apiVersions:        []string{"v1"},
		withReconcile:      true,
		reconcileWorkers:   3,
		reconcileRequeueMs: 5,
		storageType:        "file",
		dbDriver:           "sqlite",
	}

	if err := runInit(root, opts); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	requiredFiles := []string{
		filepath.Join(root, "cmd/server/main.go"),
		filepath.Join(root, "cmd/server/runtime_helpers_generated.go"),
		filepath.Join(root, "cmd/server/auth_helpers_generated.go"),
		filepath.Join(root, "cmd/server/metrics_helpers_generated.go"),
	}
	for _, file := range requiredFiles {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("expected generated scaffold file %s: %v", file, err)
		}
	}

	mainPath := filepath.Join(root, "cmd/server/main.go")
	mainContentBytes, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("ReadFile(main.go): %v", err)
	}
	mainContent := string(mainContentBytes)

	for _, marker := range []string{
		"initializeStorage(config)",
		"initializeEventingAndReconciliation(config)",
		"initializeAuthMiddleware(config)",
		"initializeMetricsServer(config)",
		"logStartupConfiguration(config, addr)",
	} {
		if !strings.Contains(mainContent, marker) {
			t.Fatalf("generated main.go missing orchestration marker %q", marker)
		}
	}

	for _, marker := range []string{
		"storage.InitFileBackend(config.DataDir)",
		"tokensmithauthn.Middleware(tokensmithauthn.Options{",
		"events.NewInMemoryEventBus(1000, 10)",
		"func startMetricsServer(",
	} {
		if strings.Contains(mainContent, marker) {
			t.Fatalf("generated main.go should not contain feature implementation marker %q", marker)
		}
	}

	fset := token.NewFileSet()
	for _, path := range requiredFiles {
		if _, err := parser.ParseFile(fset, path, nil, parser.AllErrors); err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
	}
}

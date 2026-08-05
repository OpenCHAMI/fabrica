// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
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

func TestInitScaffold_CustomStorageIsProjectOwned(t *testing.T) {
	root := filepath.Join(t.TempDir(), "custom-storage")
	opts := &initOptions{
		modulePath:         "github.com/example/custom-storage",
		description:        "custom storage smoke",
		withStorage:        true,
		withVersion:        true,
		validationMode:     "strict",
		eventBusType:       "memory",
		apiGroup:           "example.fabrica.dev",
		storageVersion:     "v1",
		apiVersions:        []string{"v1"},
		reconcileWorkers:   5,
		reconcileRequeueMs: 5,
		storageType:        "custom",
		dbDriver:           "bogus",
	}

	if err := runInit(root, opts); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	storagePath := filepath.Join(root, "internal/storage/storage.go")
	storageContent, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("ReadFile(storage.go): %v", err)
	}
	if !strings.Contains(string(storageContent), "project-owned storage implementation") {
		t.Fatalf("custom storage stub does not describe project ownership:\n%s", storageContent)
	}
	if _, err := os.Stat(filepath.Join(root, "internal/storage/ent")); !os.IsNotExist(err) {
		t.Fatalf("custom storage should not create Ent directories, stat err=%v", err)
	}

	runtimePath := filepath.Join(root, "cmd/server/runtime_helpers_generated.go")
	runtimeContent, err := os.ReadFile(runtimePath)
	if err != nil {
		t.Fatalf("ReadFile(runtime_helpers_generated.go): %v", err)
	}
	if !strings.Contains(string(runtimeContent), "Custom storage selected") {
		t.Fatalf("runtime helper does not acknowledge custom storage:\n%s", runtimeContent)
	}
	if strings.Contains(string(runtimeContent), "internal/storage/ent") || strings.Contains(string(runtimeContent), "InitFileBackend") {
		t.Fatalf("custom runtime helper should not initialize generated storage:\n%s", runtimeContent)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, runtimePath, nil, parser.AllErrors); err != nil {
		t.Fatalf("ParseFile(runtime helper): %v", err)
	}
}

func TestValidateInitOptions_RejectsUnsupportedCombinations(t *testing.T) {
	base := &initOptions{
		withStorage:  true,
		withEvents:   true,
		eventBusType: "memory",
	}

	if err := validateInitOptions(base); err != nil {
		t.Fatalf("valid init options rejected: %v", err)
	}

	noStorage := *base
	noStorage.withStorage = false
	if err := validateInitOptions(&noStorage); err == nil {
		t.Fatalf("expected storage-disabled init to be rejected")
	}

	nats := *base
	nats.eventBusType = "nats"
	if err := validateInitOptions(&nats); err == nil {
		t.Fatalf("expected unsupported event bus to be rejected")
	}

	reconcileNoEvents := *base
	reconcileNoEvents.withEvents = false
	reconcileNoEvents.withReconcile = true
	if err := validateInitOptions(&reconcileNoEvents); err == nil {
		t.Fatalf("expected reconciliation without events to be rejected")
	}

	badStorage := *base
	badStorage.storageType = "oracle"
	if err := validateInitOptions(&badStorage); err == nil {
		t.Fatalf("expected unsupported storage type to be rejected")
	}
}

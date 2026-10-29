// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
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

// writeResourceParts writes the given file name/content pairs into a fresh dir.
func writeResourceParts(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func newInitOnlyProject(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "svc")
	opts := &initOptions{
		modulePath:     "github.com/example/svc",
		withStorage:    true,
		withVersion:    true,
		validationMode: "strict",
		eventBusType:   "memory",
		apiGroup:       "example.fabrica.dev",
		storageVersion: "v1",
		apiVersions:    []string{"v1"},
		storageType:    "file",
		dbDriver:       "sqlite",
	}
	if err := runInit(root, opts); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	return root
}

func TestAddResourcesToProject_MergesAndRegisters(t *testing.T) {
	parts := writeResourceParts(t, map[string]string{
		"user.go": `package structs

// UserSpec is a user account.
//
// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:index:fields=Username,Email:name=idx_user_login:unique:type=btree
type UserSpec struct {
	// Username is the unique login name.
	// +fabrica:field:unique
	Username string ` + "`json:\"username\"`" + `
	Email    string ` + "`json:\"email\"`" + `
}
`,
		"device.go": `package structs

import (
	"context"
	"encoding/json"
)

// DeviceSpec describes a device.
type DeviceSpec struct {
	ID   string          ` + "`json:\"id\"`" + `
	Meta json.RawMessage ` + "`json:\"meta,omitempty\"`" + `
	Loc  DeviceLocation  ` + "`json:\"loc\"`" + `
}

// DeviceLocation is where a device sits.
type DeviceLocation struct {
	Rack string ` + "`json:\"rack\"`" + `
}

// Validate checks the device.
func (r *Device) Validate(ctx context.Context) error {
	// custom device validation
	return nil
}
`,
		// Helper-only file: no *Spec, must be skipped (not a resource).
		"shared.go": `package structs

type Shared struct {
	Name string ` + "`json:\"name\"`" + `
}
`,
	})

	root := newInitOnlyProject(t)

	discovered, err := discoverResourceParts(parts)
	if err != nil {
		t.Fatalf("discoverResourceParts: %v", err)
	}
	if len(discovered) != 2 {
		t.Fatalf("expected 2 resources discovered (Device, User), got %d: %+v", len(discovered), discovered)
	}
	if err := addResourcesToProject(root, discovered); err != nil {
		t.Fatalf("addResourcesToProject: %v", err)
	}

	v1 := filepath.Join(root, "apis", "example.fabrica.dev", "v1")

	// apis.yaml should register both resources (not the helper-only Shared).
	apisYAML := readTestFile(t, filepath.Join(root, "apis.yaml"))
	for _, want := range []string{"Device", "User"} {
		if !strings.Contains(apisYAML, want) {
			t.Fatalf("apis.yaml missing resource %q:\n%s", want, apisYAML)
		}
	}
	if strings.Contains(apisYAML, "Shared") {
		t.Fatalf("apis.yaml should not register helper-only Shared:\n%s", apisYAML)
	}

	// user_types.go: resource-level annotations copied onto the envelope,
	// custom Spec field and its comment preserved, no scaffold Description.
	user := readTestFile(t, filepath.Join(v1, "user_types.go"))
	for _, want := range []string{
		"// +fabrica:resource",
		"// +fabrica:storage=dedicated",
		"idx_user_login",
		"// Username is the unique login name.",
		"// +fabrica:field:unique",
		"Username string",
	} {
		if !strings.Contains(user, want) {
			t.Fatalf("user_types.go missing %q:\n%s", want, user)
		}
	}
	if strings.Contains(user, "Description string") {
		t.Fatalf("user_types.go should not keep the scaffold Description field:\n%s", user)
	}
	assertTestParses(t, "user_types.go", user)

	// device_types.go: helper type, custom Validate, and import preserved.
	device := readTestFile(t, filepath.Join(v1, "device_types.go"))
	for _, want := range []string{
		"type DeviceLocation struct",
		"custom device validation",
		"encoding/json",
		"json.RawMessage",
	} {
		if !strings.Contains(device, want) {
			t.Fatalf("device_types.go missing %q:\n%s", want, device)
		}
	}
	assertTestParses(t, "device_types.go", device)
}

func TestDiscoverResourceParts_EmptyDirFails(t *testing.T) {
	if _, err := discoverResourceParts(t.TempDir()); err == nil {
		t.Fatal("expected error for a directory with no *Spec files, got nil")
	}
}

func TestAddCommand_HasResourcesSubcommand(t *testing.T) {
	cmd := newAddCommand()
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "resources" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("add command missing 'resources' subcommand")
	}
}

// TestAddResourcesCommand_MergesInProject exercises the `fabrica add resources
// <dir>` command path in-process against an initialized project.
func TestAddResourcesCommand_MergesInProject(t *testing.T) {
	parts := writeResourceParts(t, map[string]string{
		"widget.go": `package structs

// WidgetSpec describes a widget.
type WidgetSpec struct {
	// Name is the widget name.
	Name string ` + "`json:\"name\"`" + `
}
`,
	})

	root := newInitOnlyProject(t)

	// Run the command from inside the project directory, like a user would.
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prev) }()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	cmd := newAddCommand()
	cmd.SetArgs([]string{"resources", parts})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add resources: %v", err)
	}

	widget := readTestFile(t, filepath.Join(root, "apis", "example.fabrica.dev", "v1", "widget_types.go"))
	for _, want := range []string{"type WidgetSpec struct", "// Name is the widget name.", "Name string"} {
		if !strings.Contains(widget, want) {
			t.Fatalf("widget_types.go missing %q:\n%s", want, widget)
		}
	}
	assertTestParses(t, "widget_types.go", widget)

	apisYAML := readTestFile(t, filepath.Join(root, "apis.yaml"))
	if !strings.Contains(apisYAML, "Widget") {
		t.Fatalf("apis.yaml missing Widget:\n%s", apisYAML)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertTestParses(t *testing.T, name, src string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), name, src, parser.AllErrors); err != nil {
		t.Fatalf("generated %s does not parse: %v\n%s", name, err, src)
	}
}

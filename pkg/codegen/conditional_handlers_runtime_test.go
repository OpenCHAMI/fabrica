// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedReadHandlersExecuteConditionalRequests(t *testing.T) {
	dir := t.TempDir()
	copyConditionalHandlerFixture(t, filepath.Join("testdata", "conditional_handlers_runtime"), dir)

	gen := NewGenerator(dir, "main", "example.com/test")
	gen.Config.ConditionalEnabled = true
	gen.Config.VersioningEnabled = true
	gen.Resources = []ResourceMetadata{{
		Name:         "Node",
		PluralName:   "nodes",
		Package:      "example.com/test/apis/v1",
		PackageAlias: "v1",
		TypeName:     "*v1.Node",
		SpecType:     "v1.NodeSpec",
		StatusType:   "v1.NodeStatus",
		URLPath:      "/nodes",
		StorageName:  "Node",
	}}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if err := gen.GenerateHandlers(); err != nil {
		t.Fatalf("GenerateHandlers: %v", err)
	}

	var middleware bytes.Buffer
	data := gen.middlewareData("middleware/conditional.go.tmpl")
	if err := gen.Templates["middlewareConditional"].Execute(&middleware, data); err != nil {
		t.Fatalf("render conditional middleware: %v", err)
	}
	middlewareDir := filepath.Join(dir, "internal", "middleware")
	if err := os.MkdirAll(middlewareDir, 0o755); err != nil {
		t.Fatalf("create middleware directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(middlewareDir, "conditional_generated.go"), middleware.Bytes(), 0o644); err != nil {
		t.Fatalf("write generated middleware: %v", err)
	}

	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.6")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("execute generated handlers: %v\n%s", err, output)
	}
}

func copyConditionalHandlerFixture(t *testing.T, source, destination string) {
	t.Helper()

	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		target = strings.TrimSuffix(target, ".fixture")
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		t.Fatalf("copy generated-handler fixture: %v", err)
	}
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRevisionImportExportGeneratedRuntimeFailsClosed(t *testing.T) {
	dir := t.TempDir()
	gen := revisionTestGenerator(dir)
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if err := gen.GenerateExportCommand(); err != nil {
		t.Fatalf("GenerateExportCommand() error = %v", err)
	}
	if err := gen.GenerateImportCommand(); err != nil {
		t.Fatalf("GenerateImportCommand() error = %v", err)
	}

	writeRevisionImportExportFixture(t, dir)
	for _, args := range [][]string{{"mod", "tidy"}, {"test", "-count=1", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.6")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("execute generated import/export go %v: %v\n%s", args, err, output)
		}
	}
}

func TestRevisionImportExportGeneratedAllRevisionCompiles(t *testing.T) {
	dir := t.TempDir()
	gen := revisionTestGenerator(dir)
	for i := range gen.Resources {
		gen.Resources[i].RevisioningEnabled = true
	}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if err := gen.GenerateExportCommand(); err != nil {
		t.Fatalf("GenerateExportCommand() error = %v", err)
	}
	if err := gen.GenerateImportCommand(); err != nil {
		t.Fatalf("GenerateImportCommand() error = %v", err)
	}

	writeRevisionImportExportFixture(t, dir)
	for _, args := range [][]string{{"mod", "tidy"}, {"test", "-run", "^$", "-count=1", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.6")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("compile all-revision generated import/export go %v: %v\n%s", args, err, output)
		}
	}
}

func writeRevisionImportExportFixture(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"go.mod": `module example.com/revisions

go 1.26.6

require (
	github.com/spf13/cobra v1.10.2
	gopkg.in/yaml.v3 v3.0.1
)
`,
		"apis/v1/types.go": `package v1

type Metadata struct { UID string ` + "`json:\"uid\" yaml:\"uid\"`" + ` }
type Node struct { APIVersion string ` + "`json:\"apiVersion\" yaml:\"apiVersion\"`" + `; Kind string ` + "`json:\"kind\" yaml:\"kind\"`" + `; Metadata Metadata ` + "`json:\"metadata\" yaml:\"metadata\"`" + `; UID string ` + "`json:\"-\" yaml:\"-\"`" + ` }
type BootConfig struct { APIVersion string ` + "`json:\"apiVersion\" yaml:\"apiVersion\"`" + `; Kind string ` + "`json:\"kind\" yaml:\"kind\"`" + `; Metadata Metadata ` + "`json:\"metadata\" yaml:\"metadata\"`" + ` }
`,
		"internal/storage/storage.go": `package storage

import (
	"context"
	"fmt"

	v1 "example.com/revisions/apis/v1"
)

var nodes = map[string]*v1.Node{}

type NodeQuery struct{}

func Querynodes(context.Context) *NodeQuery { return &NodeQuery{} }
func (*NodeQuery) All(context.Context) ([]*v1.Node, error) {
	result := make([]*v1.Node, 0, len(nodes))
	for _, node := range nodes { result = append(result, node) }
	return result, nil
}
func GetNodeByUID(_ context.Context, uid string) (*v1.Node, error) {
	node, ok := nodes[uid]
	if !ok { return nil, fmt.Errorf("node not found") }
	return node, nil
}
func SaveNode(_ context.Context, node *v1.Node) error { node.UID = node.Metadata.UID; nodes[node.Metadata.UID] = node; return nil }
func DeleteNode(_ context.Context, uid string) error { delete(nodes, uid); return nil }
func ResetNodes(seed ...*v1.Node) { nodes = map[string]*v1.Node{}; for _, node := range seed { node.UID = node.Metadata.UID; nodes[node.Metadata.UID] = node } }
func HasNode(uid string) bool { _, ok := nodes[uid]; return ok }
`,
		"import_export_test.go": `package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "example.com/revisions/apis/v1"
	"example.com/revisions/internal/storage"
)

func TestExportMixedRevisionKindsHasNoSideEffects(t *testing.T) {
	storage.ResetNodes(&v1.Node{APIVersion:"v1", Kind:"Node", Metadata:v1.Metadata{UID:"existing"}})
	output := filepath.Join(t.TempDir(), "backup")

	err := runExport(context.Background(), "json", output, []string{"Node", "BootConfig"}, true)

	if err == nil { t.Fatal("runExport() error = nil, want unsupported revision error") }
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) { t.Fatalf("output side effect: os.Stat() error = %v", statErr) }
}

func TestImportMixedRevisionFilesHasNoSideEffects(t *testing.T) {
	storage.ResetNodes(&v1.Node{APIVersion:"v1", Kind:"Node", Metadata:v1.Metadata{UID:"existing"}})
	input := t.TempDir()
	files := map[string]string{
		"00-revision.json": ` + "`{\"apiVersion\":\"v1\",\"kind\":\"BootConfig\",\"metadata\":{\"uid\":\"revision\"}}`" + `,
		"10-node.json": ` + "`{\"apiVersion\":\"v1\",\"kind\":\"Node\",\"metadata\":{\"uid\":\"new\"}}`" + `,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(input, name), []byte(content), 0o644); err != nil { t.Fatal(err) }
	}

	err := runImport(context.Background(), input, "replace", false, false)

	if err == nil { t.Fatal("runImport() error = nil, want unsupported revision error") }
	if !storage.HasNode("existing") { t.Error("existing mutable resource was deleted") }
	if storage.HasNode("new") { t.Error("ordinary resource was imported after revision preflight failure") }
}
`,
	}
	for relative, content := range files {
		path := filepath.Join(dir, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", relative, err)
		}
	}
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGeneratedClientPathSegments(t *testing.T) {
	dir := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": fmt.Sprintf(`module example.com/clientpaths

go 1.26.10

require (
	github.com/openchami/fabrica v0.0.0
	github.com/rs/zerolog v1.35.1
	github.com/spf13/cobra v1.10.2
)

replace github.com/openchami/fabrica => %s
`, filepath.ToSlash(root)),
		"apis/v1/types.go": `package v1
import "github.com/openchami/fabrica/pkg/fabrica"
type NodeSpec struct{}
type NodeStatus struct{}
type Node struct {
	Metadata fabrica.Metadata
	Spec NodeSpec
	Status NodeStatus
}
type BootConfigSpec struct{}
type BootConfigStatus struct{}
type BootConfig struct {
	Metadata fabrica.Metadata
	Spec BootConfigSpec
	Status BootConfigStatus
}
`,
	}
	for name, content := range files {
		filename := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gen := NewGenerator(dir, "client", "example.com/clientpaths")
	gen.Resources = []ResourceMetadata{
		{
			Name: "Node", PluralName: "nodes", Package: "example.com/clientpaths/apis/v1", PackageAlias: "v1",
			TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", URLPath: "/api/v1/nodes",
			Tags: map[string]string{"versioning": "enabled"},
		},
		{
			Name: "BootConfig", PluralName: "bootConfigs", Package: "example.com/clientpaths/apis/v1", PackageAlias: "v1",
			TypeName: "*v1.BootConfig", SpecType: "v1.BootConfigSpec", StatusType: "v1.BootConfigStatus", URLPath: "/api/v1/boot-configs",
			RevisioningEnabled: true,
		},
	}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	if err := gen.GenerateClient(); err != nil {
		t.Fatal(err)
	}
	if err := gen.GenerateClientModels(); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "client_path_segments_test.go.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "client_paths_test.go"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"mod", "tidy"}, {"test", "-race", "-count=1", "."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.10")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("generated client go %v: %v\n%s", args, err, output)
		}
		t.Logf("generated client go %v:\n%s", args, output)
	}
}

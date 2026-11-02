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

// Generate the server, storage, client, and CLI together: independently rendered
// templates cannot establish that clients expose the operations the server serves.
func TestRevisionGeneratedService(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixtureDir := filepath.Join(root, "pkg", "codegen", "testdata", "revision_service_runtime")
	for _, mixed := range []bool{false, true} {
		name := "revision_only"
		if mixed {
			name = "mixed"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			writeServiceFixture := func(relative string, data []byte) {
				t.Helper()
				path := filepath.Join(dir, relative)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			writeServiceFixture("go.mod", []byte(fmt.Sprintf(`module example.com/revisionservice

go 1.26.10

require (
	github.com/openchami/fabrica v0.0.0
	github.com/getkin/kin-openapi v0.151.0
	github.com/go-chi/chi/v5 v5.5.0
	github.com/rs/zerolog v1.35.1
	github.com/spf13/viper v1.21.0
)

replace github.com/openchami/fabrica => %s
`, filepath.ToSlash(root))))
			for _, file := range []struct{ source, target string }{
				{"types.go.fixture", "apis/v1/types.go"},
				{"workflow_test.go.fixture", "cmd/server/workflow_test.go"},
				{"commands_test.go.fixture", "cmd/client/commands_test.go"},
			} {
				data, err := os.ReadFile(filepath.Join(fixtureDir, file.source))
				if err != nil {
					t.Fatal(err)
				}
				writeServiceFixture(file.target, data)
			}
			resources := []ResourceMetadata{{
				Name: "BootConfig", PluralName: "bootConfigs", Package: "example.com/revisionservice/apis/v1", PackageAlias: "v1",
				TypeName: "*v1.BootConfig", SpecType: "v1.BootConfigSpec", StatusType: "v1.BootConfigStatus", URLPath: "/boot-configs",
				StorageName: "BootConfig", RevisioningEnabled: true, BareNameSelector: "default",
			}}
			if mixed {
				data, err := os.ReadFile(filepath.Join(fixtureDir, "mutable_test.go.fixture"))
				if err != nil {
					t.Fatal(err)
				}
				writeServiceFixture("cmd/server/mutable_test.go", data)
				writeServiceFixture("cmd/client/mutable_test.go", []byte(`package main
import "testing"
func TestMutableCommandsRemainAvailable(t *testing.T) {
 for _, name := range []string{"list", "get", "create", "update", "patch", "delete"} {
  command, remaining, err := rootCmd.Find([]string{"node", name})
  if err != nil || len(remaining) != 0 || command.Name() != name { t.Errorf("ordinary command %s missing", name) }
 }
}
`))
				resources = append(resources, ResourceMetadata{
					Name: "Node", PluralName: "nodes", Package: "example.com/revisionservice/apis/v1", PackageAlias: "v1",
					TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", URLPath: "/nodes", StorageName: "Node",
				})
			}
			server := NewGenerator(filepath.Join(dir, "cmd", "server"), "main", "example.com/revisionservice")
			server.Config.VersioningEnabled = true
			server.Config.ConditionalEnabled = true
			server.Resources = resources
			if err := server.LoadTemplates(); err != nil {
				t.Fatal(err)
			}
			for _, generate := range []func() error{server.GenerateModels, server.GenerateHandlers, server.GenerateRoutes, server.GenerateOpenAPI, server.GenerateStorage, server.GenerateMiddleware} {
				if err := generate(); err != nil {
					t.Fatal(err)
				}
			}
			client := NewGenerator(filepath.Join(dir, "pkg", "client"), "client", "example.com/revisionservice")
			client.Resources = resources
			if err := client.LoadTemplates(); err != nil {
				t.Fatal(err)
			}
			for _, generate := range []func() error{client.GenerateClient, client.GenerateClientModels, client.GenerateClientCmd, client.GenerateClientVersion} {
				if err := generate(); err != nil {
					t.Fatal(err)
				}
			}
			for _, args := range [][]string{{"mod", "tidy"}, {"test", "-race", "-count=1", "./..."}} {
				cmd := exec.Command("go", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.10")
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("generated %s service: go %v: %v\n%s", name, args, err, output)
				}
			}
		})
	}
}

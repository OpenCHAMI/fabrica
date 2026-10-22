// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Compile the actual runner against a pre-loader generator API. String assertions
// alone cannot catch references to symbols absent from older generator modules.
func TestFeatureLoaderRunnerSupportsLegacyGenerator(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("old/go.mod", "module github.com/openchami/fabrica\n\ngo 1.26.10\n")
	write("old/pkg/codegen/generator.go", legacyGeneratorFixture)
	write("go.mod", `module example.com/legacy

go 1.26.10

require (
 github.com/openchami/fabrica v0.0.0
 gopkg.in/yaml.v3 v3.0.1
)
replace github.com/openchami/fabrica => ./old
`)
	write("pkg/resources/register.go", `package resources
import "github.com/openchami/fabrica/pkg/codegen"
func RegisterAllResources(*codegen.Generator) error {return nil}
`)
	write("cmd/server/main.go", "package main\ntype Config struct{}\n")
	write(".fabrica.yaml", "features:\n  storage:\n    type: file\n")
	write("runner.go", generateRunnerCode(root, "example.com/legacy", "cmd/server", "main", true, false, false, false, false, "file"))
	run := func() ([]byte, error) {
		cmd := exec.Command("go", "run", "-mod=mod", "runner.go")
		cmd.Dir = root
		return cmd.CombinedOutput()
	}
	out, err := run()
	if err != nil {
		t.Fatalf("legacy generator runner: %v\n%s", err, out)
	}
	for _, name := range []string{"middleware.called", "metrics.called"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	// An adopted scaffold must fail explicitly when its generator is too old.
	write("cmd/server/main.go", "package main\ntype Config struct{GeneratedFeatureConfig}\n")
	out, err = run()
	if err == nil || !strings.Contains(string(out), "require a Fabrica generator with GenerateFeatures support") {
		t.Fatalf("old generator accepted adopted service: %v\n%s", err, out)
	}
}

const legacyGeneratorFixture = `package codegen
import "os"
type Config struct {
 ValidationEnabled,ConditionalEnabled,EventsEnabled,MetricsEnabled,VersioningEnabled bool
 ValidationMode,ETagAlgorithm,EventBusType,OpenAPIServerURL,OpenAPIServerDescription,StorageType,DBDriver string
 WithAuth,SecurityAuthNEnabled bool
}
type Generator struct {
 OutputDir,StorageType,DBDriver,Version,Commit string
 Config *Config
 Verbose bool
}
func NewGenerator(output,pkg,module string)*Generator{return &Generator{OutputDir:output,Config:&Config{}}}
func(g *Generator)SetStorageType(s string){g.StorageType=s}
func(g *Generator)SetDBDriver(s string){g.DBDriver=s}
func(g *Generator)LoadTemplates()error{return nil}
func(g *Generator)GenerateHandlers()error{return nil}
func(g *Generator)GenerateMiddleware()error{return os.WriteFile("middleware.called",nil,0644)}
func(g *Generator)GenerateMetrics()error{return os.WriteFile("metrics.called",nil,0644)}
func(g *Generator)GenerateServerVersion()error{return nil}
func(g *Generator)GenerateRoutes()error{return nil}
func(g *Generator)GenerateModels()error{return nil}
func(g *Generator)GenerateAPIVersions()error{return nil}
`

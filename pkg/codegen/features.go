// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openchami/fabrica/internal/constants"
)

// FeatureConfig is the generation-time startup contract shared by init and generate.
// Runtime configuration can configure only the capabilities selected here.
type FeatureConfig struct {
	ProjectName             string
	StorageType             string
	DBDriver                string
	EventsEnabled           bool
	EventBusType            string
	ReconciliationEnabled   bool
	ReconcileWorkers        int
	ReconcileRequeueMinutes int
	MetricsEnabled          bool
	AuthEnabled             bool
	AuthZEnabled            bool
	AuthZMode               string
	ValidationEnabled       bool
	ValidationMode          string
	ConditionalEnabled      bool
	ETagAlgorithm           string
	VersioningEnabled       bool
}

// Normalize supplies existing defaults without enabling optional capabilities.
func (f FeatureConfig) Normalize() FeatureConfig {
	if f.StorageType == "" {
		f.StorageType = "file"
	}
	if f.DBDriver == "" || f.DBDriver == "sqlite" {
		f.DBDriver = "sqlite3"
	}
	if f.EventBusType == "" {
		f.EventBusType = "memory"
	}
	if f.ReconcileWorkers == 0 {
		f.ReconcileWorkers = 5
	}
	if f.ReconcileRequeueMinutes == 0 {
		f.ReconcileRequeueMinutes = 5
	}
	if f.AuthZMode != "shadow" && f.AuthZMode != "enforce" {
		f.AuthZMode = "enforce"
	}
	if f.ValidationMode == "disabled" {
		f.ValidationEnabled = false
	}
	return f
}

// Validate rejects unsupported startup selections before generation mutates files.
func (f FeatureConfig) Validate() error {
	f = f.Normalize()
	if f.ReconciliationEnabled && !f.EventsEnabled {
		return fmt.Errorf("reconciliation requires events")
	}
	if f.AuthZEnabled && !f.AuthEnabled {
		return fmt.Errorf("authorization requires authentication")
	}
	if f.EventsEnabled && f.EventBusType != "memory" {
		return fmt.Errorf("unsupported event bus %q", f.EventBusType)
	}
	if f.StorageType != "file" && f.StorageType != "ent" && f.StorageType != "custom" {
		return fmt.Errorf("unsupported storage %q", f.StorageType)
	}
	if f.StorageType == "ent" && f.DBDriver != "sqlite3" && f.DBDriver != "postgres" && f.DBDriver != "mysql" {
		return fmt.Errorf("unsupported database driver %q", f.DBDriver)
	}
	if f.ReconcileWorkers < 1 || f.ReconcileRequeueMinutes < 1 {
		return fmt.Errorf("reconciliation workers and requeue delay must be positive")
	}
	return nil
}

// SetFeatures sets the complete generation model, including legacy generator toggles.
func (g *Generator) SetFeatures(f FeatureConfig) {
	f = f.Normalize()
	g.Config.Features = f
	g.SetStorageType(f.StorageType)
	g.SetDBDriver(f.DBDriver)
	g.Config.StorageType, g.Config.DBDriver = f.StorageType, f.DBDriver
	g.SetAuthEnabled(f.AuthEnabled)
	g.Config.EventsEnabled, g.Config.EventBusType = f.EventsEnabled, f.EventBusType
	g.Config.MetricsEnabled = f.MetricsEnabled
	g.Config.ValidationEnabled, g.Config.ValidationMode = f.ValidationEnabled, f.ValidationMode
	g.Config.ConditionalEnabled, g.Config.ETagAlgorithm = f.ConditionalEnabled, f.ETagAlgorithm
	g.Config.VersioningEnabled = f.VersioningEnabled
}

// UsesFeatureLoader detects adoption structurally, without depending on comments.
func UsesFeatureLoader(serverDir string) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(serverDir, "main.go"), nil, 0)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	embedded, called := false, false
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.TypeSpec:
			if n.Name.Name == "Config" {
				if s, ok := n.Type.(*ast.StructType); ok {
					for _, field := range s.Fields.List {
						if id, ok := field.Type.(*ast.Ident); ok && len(field.Names) == 0 && id.Name == "GeneratedFeatureConfig" {
							embedded = true
						}
					}
				}
			}
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "loadGeneratedFeatures" {
				called = true
			}
		}
		return true
	})
	if embedded != called {
		return false, fmt.Errorf("incomplete feature-loader migration: embed GeneratedFeatureConfig and call loadGeneratedFeatures; see docs/guides/feature-loader.md")
	}
	return embedded && called, nil
}

const featureManifestName = "features_manifest_generated.json"

type featureManifest struct {
	Format int
	Config FeatureConfig
	Files  map[string]string
}

type featureOutput struct {
	path, template string
	enabled        bool
	data           interface{}
}

type featureTemplateData struct {
	FeatureConfig
	WithStorage, WithEvents, WithReconcile, WithAuth  bool
	ModulePath, MetricsNamespace                      string
	FabricaVersion, GeneratedAt, TokenSmithModulePath string
	CopyrightYear                                     int
}

func (g *Generator) featureData() featureTemplateData {
	f := g.Config.Features.Normalize()
	if f.ProjectName == "" {
		f.ProjectName = g.extractServiceName()
	}
	common := g.commonTemplateData("server/features.go.tmpl")
	return featureTemplateData{
		FeatureConfig: f,
		WithStorage:   true, WithEvents: f.EventsEnabled, WithReconcile: f.ReconciliationEnabled, WithAuth: f.AuthEnabled,
		ModulePath:       g.ModulePath,
		MetricsNamespace: strings.NewReplacer("-", "_", ".", "_").Replace(f.ProjectName),
		FabricaVersion:   g.Version, GeneratedAt: common["GeneratedAt"].(string), CopyrightYear: common["CopyrightYear"].(int), TokenSmithModulePath: constants.TokenSmithModulePath,
	}
}

func (g *Generator) featureOutputs() []featureOutput {
	d := g.featureData()
	f := d.FeatureConfig
	middlewareDir := filepath.Clean(filepath.Join(g.OutputDir, "..", "..", "internal", "middleware"))
	return []featureOutput{
		{filepath.Join(g.OutputDir, "features_generated.go"), "features", true, d},
		{filepath.Join(g.OutputDir, "runtime_helpers_generated.go"), "runtimeHelpers", true, d},
		{filepath.Join(g.OutputDir, "auth_helpers_generated.go"), "authHelpers", f.AuthEnabled, d},
		{filepath.Join(g.OutputDir, "metrics_generated.go"), "metrics", f.MetricsEnabled, d},
		{filepath.Join(g.OutputDir, "metrics_helpers_generated.go"), "", false, nil},
		{filepath.Join(middlewareDir, "validation_middleware_generated.go"), "middlewareValidation", f.ValidationEnabled, g.middlewareData("middleware/validation.go.tmpl")},
		{filepath.Join(middlewareDir, "conditional_middleware_generated.go"), "middlewareConditional", f.ConditionalEnabled, g.middlewareData("middleware/conditional.go.tmpl")},
		{filepath.Join(middlewareDir, "versioning_middleware_generated.go"), "middlewareVersioning", f.VersioningEnabled, g.middlewareData("middleware/versioning.go.tmpl")},
		{filepath.Join(middlewareDir, "event_bus_generated.go"), "eventBus", f.EventsEnabled, g.middlewareData("middleware/event-bus.go.tmpl")},
	}
}

func featureDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// CheckFeatureGeneration performs ownership and selective-generation checks before
// the CLI mutates any output. Changed feature selections require a full generation.
func (g *Generator) CheckFeatureGeneration(full bool) error {
	adopted, err := UsesFeatureLoader(g.OutputDir)
	if err != nil || !adopted {
		return err
	}
	if err := g.Config.Features.Validate(); err != nil {
		return err
	}
	var previous featureManifest
	b, err := os.ReadFile(filepath.Join(g.OutputDir, featureManifestName))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err := json.Unmarshal(b, &previous); err != nil {
			return fmt.Errorf("invalid feature manifest: %w", err)
		}
		if previous.Format != 1 {
			return fmt.Errorf("unsupported feature manifest format %d", previous.Format)
		}
	}
	if !full && (previous.Format == 0 || previous.Config != g.Config.Features.Normalize()) {
		return fmt.Errorf("feature configuration changed; run full 'fabrica generate' before selective generation")
	}
	// Only consult fixed known paths. Never trust manifest paths for deletion.
	for _, output := range g.featureOutputs() {
		existing, err := os.ReadFile(output.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(g.OutputDir, output.path)
		if err != nil {
			return err
		}
		digest, tracked := previous.Files[filepath.ToSlash(relative)]
		if !tracked || digest != featureDigest(existing) {
			return fmt.Errorf("feature output %s is untracked or modified; preserve customizations and move it aside before regeneration (see docs/guides/feature-loader.md)", output.path)
		}
	}
	return nil
}

// GenerateFeatures emits the enabled startup composition and removes obsolete,
// previously tracked feature outputs. Legacy scaffolds retain their old helpers.
func (g *Generator) GenerateFeatures() error {
	adopted, err := UsesFeatureLoader(g.OutputDir)
	if err != nil {
		return err
	}
	if !adopted {
		fmt.Println("Legacy startup scaffold: init-time feature limitations remain; see docs/guides/feature-loader.md to migrate")
		if err := g.GenerateMiddleware(); err != nil {
			return err
		}
		return g.GenerateMetrics()
	}
	f := g.Config.Features.Normalize()
	if err := g.CheckFeatureGeneration(true); err != nil {
		return err
	}
	// Render and format everything before changing any output.
	outputs := g.featureOutputs()
	rendered := make(map[string][]byte)
	for _, output := range outputs {
		if !output.enabled {
			continue
		}
		tmpl := g.Templates[output.template]
		if tmpl == nil {
			return fmt.Errorf("feature template %s not loaded", output.template)
		}
		var b bytes.Buffer
		if err := tmpl.Execute(&b, output.data); err != nil {
			return err
		}
		formatted, err := format.Source(b.Bytes())
		if err != nil {
			return fmt.Errorf("format %s: %w", output.path, err)
		}
		rendered[output.path] = formatted
	}
	manifest := featureManifest{Format: 1, Config: f, Files: make(map[string]string)}
	for _, output := range outputs {
		if !output.enabled {
			if err := os.Remove(output.path); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(output.path), 0755); err != nil {
			return err
		}
		if _, err := g.writeGeneratedFile(output.path, rendered[output.path]); err != nil {
			return err
		}
		actual, err := os.ReadFile(output.path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(g.OutputDir, output.path)
		if err != nil {
			return err
		}
		manifest.Files[filepath.ToSlash(relative)] = featureDigest(actual)
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	_, err = writeGeneratedFile(filepath.Join(g.OutputDir, featureManifestName), append(b, '\n'))
	return err
}

// GenerateFeaturesIfAdopted refreshes startup for storage-only generation while
// leaving legacy startup helpers alone.
func (g *Generator) GenerateFeaturesIfAdopted() error {
	adopted, err := UsesFeatureLoader(g.OutputDir)
	if err != nil || !adopted {
		return err
	}
	return g.GenerateFeatures()
}

// FeatureImpact reports selected writes and existing obsolete outputs without
// changing files. Paths are relative to the service root (cmd/server's parent).
func (g *Generator) FeatureImpact() (writes, removals []string, err error) {
	root := filepath.Clean(filepath.Join(g.OutputDir, "..", ".."))
	for _, output := range g.featureOutputs() {
		path, err := filepath.Rel(root, output.path)
		if err != nil {
			return nil, nil, err
		}
		if output.enabled {
			writes = append(writes, filepath.ToSlash(path))
			continue
		}
		if _, err := os.Stat(output.path); err == nil {
			removals = append(removals, filepath.ToSlash(path))
		} else if !os.IsNotExist(err) {
			return nil, nil, err
		}
	}
	path, err := filepath.Rel(root, filepath.Join(g.OutputDir, featureManifestName))
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, filepath.ToSlash(path))
	sort.Strings(writes)
	sort.Strings(removals)
	return writes, removals, nil
}

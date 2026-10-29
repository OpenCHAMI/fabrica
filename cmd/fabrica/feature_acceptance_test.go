// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	configpkg "github.com/openchami/fabrica/internal/config"
	"github.com/openchami/fabrica/internal/constants"
	"github.com/openchami/fabrica/internal/projectfeatures"
	"github.com/openchami/fabrica/pkg/codegen"
)

type featureAcceptance struct{ repo, workspace, binary string }
type acceptanceProject struct {
	*featureAcceptance
	dir, apiImport string
	cfg            *configpkg.FabricaConfig
	owned          map[string][]byte
}
type acceptanceProfile struct {
	Name                                                      string
	Metrics, Events, Reconcile, Auth, LegacyAuth, Conditional bool
	Algorithm                                                 string
	Workers, Minutes                                          int
}

func newFeatureAcceptance(t *testing.T) *featureAcceptance {
	t.Helper()
	if testing.Short() {
		t.Skip("generated service acceptance")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	h := &featureAcceptance{repo: repo, workspace: t.TempDir()}
	h.binary = filepath.Join(h.workspace, "fabrica")
	h.run(t, repo, true, nil, "go", "build", "-o", h.binary, "./cmd/fabrica")
	return h
}
func (h *featureAcceptance) run(t *testing.T, dir string, success bool, env []string, executable string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if (err == nil) != success {
		t.Fatalf("%s %v: %v\n%s", executable, args, err, out)
	}
	return out
}
func (h *featureAcceptance) project(t *testing.T, name, backend string, flags ...string) *acceptanceProject {
	t.Helper()
	parent := filepath.Join(h.workspace, name)
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	initArgs := []string{"init", "acceptance", "--module", "example.com/acceptance", "--storage-type", backend}
	h.run(t, parent, true, nil, h.binary, append(initArgs, flags...)...)
	p := &acceptanceProject{featureAcceptance: h, dir: filepath.Join(parent, "acceptance"), owned: make(map[string][]byte)}
	h.run(t, p.dir, true, nil, "go", "mod", "edit", "-replace", "github.com/openchami/fabrica="+h.repo)
	h.run(t, p.dir, true, nil, h.binary, "add", "resource", "Node")
	paths, err := filepath.Glob(filepath.Join(p.dir, "apis", "*", "*", "node_types.go"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("resource inputs: %v %v", paths, err)
	}
	apiDir, err := filepath.Rel(p.dir, filepath.Dir(paths[0]))
	if err != nil {
		t.Fatal(err)
	}
	p.apiImport = "example.com/acceptance/" + filepath.ToSlash(apiDir)
	p.cfg, err = configpkg.LoadConfig(p.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(p.dir, "cmd/server/main.go"), paths[0]} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		p.owned[path] = data
	}
	return p
}
func (p *acceptanceProject) configure(t *testing.T, s acceptanceProfile) {
	t.Helper()
	f := &p.cfg.Features
	f.Metrics.Enabled = s.Metrics
	f.Events.Enabled = s.Events
	f.Reconciliation.Enabled = s.Reconcile
	f.Reconciliation.WorkerCount = s.Workers
	if s.Workers == 0 {
		f.Reconciliation.WorkerCount = 5
	}
	f.Reconciliation.RequeueDelay = s.Minutes
	if s.Minutes == 0 {
		f.Reconciliation.RequeueDelay = 5
	}
	f.Auth.Enabled = s.LegacyAuth
	f.Security.AuthN.Enabled = s.Auth && !s.LegacyAuth
	f.Conditional.Enabled = s.Conditional
	f.Conditional.ETagAlgorithm = s.Algorithm
	if err := configpkg.SaveConfig(p.dir, p.cfg); err != nil {
		t.Fatal(err)
	}
}
func (p *acceptanceProject) generate(t *testing.T) {
	t.Helper()
	p.run(t, p.dir, true, nil, p.binary, "generate", "--fabrica-source", p.repo)
	// Pin the same supported dependency when auth first appears or its support is retained.
	p.run(t, p.dir, true, nil, "go", "mod", "edit", "-require", constants.TokenSmithModulePath+"@"+constants.TokenSmithVersion)
	p.run(t, p.dir, true, nil, "go", "mod", "tidy")
	for path, want := range p.owned {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(want, got) {
			t.Fatalf("generation changed user input %s: %v", path, err)
		}
	}
}
func (p *acceptanceProject) fixture(t *testing.T, name string, data any) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("testdata", "acceptance", name))
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New(name).Parse(string(source))
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(rendered.Bytes())
	if err != nil {
		t.Fatalf("format acceptance fixture %s: %v\n%s", name, err, rendered.Bytes())
	}
	path := filepath.Join(p.dir, "cmd/server/acceptance_test.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		t.Fatal(err)
	}
}
func (p *acceptanceProject) check(t *testing.T, env []string) {
	t.Helper()
	p.run(t, p.dir, true, env, "go", "test", "./cmd/server", "-count=1")
	p.run(t, p.dir, true, nil, "go", "build", "./...")
}
func acceptanceTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

var acceptanceMetadata = regexp.MustCompile(`(?m)^// (?:Generated(?: at)?: .*|Code generated by [Ff]abrica.*DO NOT EDIT\.)$`)

type acceptanceOutput struct {
	Config codegen.FeatureConfig
	Files  map[string]string
}

func (p *acceptanceProject) featureOutputs(t *testing.T) acceptanceOutput {
	t.Helper()
	server := filepath.Join(p.dir, "cmd/server")
	data, err := os.ReadFile(filepath.Join(server, "features_manifest_generated.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Format int
		Config codegen.FeatureConfig
		Files  map[string]string
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Format != 1 {
		t.Fatalf("unexpected manifest format: %d", manifest.Format)
	}
	out := acceptanceOutput{Config: manifest.Config, Files: make(map[string]string)}
	for path, digest := range manifest.Files {
		content, err := os.ReadFile(filepath.Join(server, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(content)
		if hex.EncodeToString(hash[:]) != digest {
			t.Fatalf("manifest hash drift: %s", path)
		}
		out.Files[filepath.ToSlash(path)] = string(acceptanceMetadata.ReplaceAll(content, []byte("// normalized generation metadata")))
	}
	return out
}
func (p *acceptanceProject) outputs(t *testing.T) acceptanceOutput {
	t.Helper()
	out := p.featureOutputs(t)
	// Compare API artifacts as well as loader composition. Reconciler support may
	// intentionally remain after disabling it to support application-owned code.
	for _, path := range []string{"cmd/server/node_handlers_generated.go", "cmd/server/models_generated.go", "cmd/server/routes_generated.go", "cmd/server/openapi_generated.go", "internal/storage/storage_generated.go"} {
		content, err := os.ReadFile(filepath.Join(p.dir, path))
		if err != nil {
			t.Fatal(err)
		}
		out.Files[path] = string(acceptanceMetadata.ReplaceAll(content, []byte("// normalized generation metadata")))
	}
	return out
}
func (p *acceptanceProject) reconciler(t *testing.T) {
	t.Helper()
	path := filepath.Join(p.dir, "pkg/reconcilers/node_reconciler.go")
	if _, ok := p.owned[path]; ok {
		return
	}
	source := strings.ReplaceAll(acceptanceReconciler, "API_IMPORT", p.apiImport)
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	p.owned[path] = []byte(source)
}

func (p *acceptanceProject) independentFixture(t *testing.T, state acceptanceProfile) {
	t.Helper()
	p.fixture(t, "independent_test.go.tmpl", struct {
		acceptanceProfile
		APIImport string
	}{state, p.apiImport})
	if !state.Auth {
		return
	}
	path := filepath.Join(p.dir, "cmd/server/acceptance_test.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(append(data, []byte(featureLoaderAuthRuntimeTests)...))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		t.Fatal(err)
	}
}
func acceptanceInitFlags(state acceptanceProfile) []string {
	workers, minutes := state.Workers, state.Minutes
	if workers == 0 {
		workers = 5
	}
	if minutes == 0 {
		minutes = 5
	}
	return []string{
		"--metrics=" + strconv.FormatBool(state.Metrics), "--events=" + strconv.FormatBool(state.Events),
		"--reconcile=" + strconv.FormatBool(state.Reconcile), "--auth=" + strconv.FormatBool(state.Auth),
		"--reconcile-workers=" + strconv.Itoa(workers), "--reconcile-requeue=" + strconv.Itoa(minutes),
	}
}

func TestFeatureLoaderIndependentAcceptance(t *testing.T) {
	h := newFeatureAcceptance(t)
	base := acceptanceProfile{Name: "off", Conditional: true, Algorithm: "sha256"}
	metrics := base
	metrics.Name = "metrics"
	metrics.Metrics = true
	events := base
	events.Name = "events"
	events.Events = true
	reconcile := events
	reconcile.Name = "reconcile-2-3"
	reconcile.Reconcile = true
	reconcile.Workers = 2
	reconcile.Minutes = 3
	retuned := reconcile
	retuned.Name = "reconcile-4-7"
	retuned.Workers = 4
	retuned.Minutes = 7
	unconditional := base
	unconditional.Name = "unconditional"
	unconditional.Conditional = false
	md5 := base
	md5.Name = "md5"
	md5.Algorithm = "md5"
	auth := base
	auth.Name = "auth-security"
	auth.Auth = true
	legacyAuth := auth
	legacyAuth.Name = "auth-legacy"
	legacyAuth.LegacyAuth = true
	fresh := make(map[string]acceptanceOutput)
	for _, family := range []struct {
		name   string
		states []acceptanceProfile
	}{
		{"metrics", []acceptanceProfile{base, metrics, base}},
		{"events", []acceptanceProfile{base, events, reconcile, retuned, events, base}},
		{"conditional", []acceptanceProfile{unconditional, base, md5, unconditional}},
		{"auth", []acceptanceProfile{base, auth, legacyAuth, base}},
	} {
		t.Run(family.name, func(t *testing.T) {
			p := h.project(t, "regenerated-"+family.name, "file")
			var previous *codegen.FeatureConfig
			for _, state := range family.states {
				t.Run(state.Name, func(t *testing.T) {
					p.configure(t, state)
					effective := projectfeatures.FromConfig(p.cfg, true)
					if previous != nil && *previous != effective {
						before := acceptanceTree(t, p.dir)
						out := p.run(t, p.dir, false, nil, p.binary, "generate", "--handlers", "--fabrica-source", h.repo)
						if !strings.Contains(string(out), "run full 'fabrica generate'") {
							t.Fatalf("selective transition: %s", out)
						}
						if !reflect.DeepEqual(before, acceptanceTree(t, p.dir)) {
							t.Fatal("selective rejection changed project files")
						}
					} else if previous != nil {
						// Raw legacy/security AuthN flags may differ while the effective policy
						// remains unchanged; selective generation must still be accepted.
						p.run(t, p.dir, true, nil, p.binary, "generate", "--handlers", "--fabrica-source", h.repo)
					}
					p.generate(t)
					if state.Reconcile {
						p.reconciler(t)
					}
					p.independentFixture(t, state)
					p.check(t, nil)
					actual := p.outputs(t)
					key, err := json.Marshal(effective)
					if err != nil {
						t.Fatal(err)
					}
					expected, ok := fresh[string(key)]
					if !ok {

						q := h.project(t, "fresh-"+family.name+"-"+state.Name, "file", acceptanceInitFlags(state)...)
						initialized := q.featureOutputs(t)
						// Conditional disabling and MD5 have YAML-only configuration; the init
						// flags must nevertheless select every feature they support correctly.
						expectedInit := effective
						expectedInit.ConditionalEnabled = true
						expectedInit.ETagAlgorithm = "sha256"
						if initialized.Config != expectedInit {
							t.Fatalf("init flags disagree with normalized generation: got %+v want %+v", initialized.Config, expectedInit)
						}
						q.configure(t, state)
						q.generate(t)
						if effective == expectedInit && !reflect.DeepEqual(initialized, q.featureOutputs(t)) {
							t.Fatal("first generation changed the init-selected feature composition")
						}
						if state.Reconcile {
							q.reconciler(t)
						}
						q.independentFixture(t, state)
						q.check(t, nil)
						expected = q.outputs(t)
						fresh[string(key)] = expected
					}
					if !reflect.DeepEqual(expected, actual) {
						for path, want := range expected.Files {
							if actual.Files[path] != want {
								t.Errorf("fresh/regenerated output differs: %s", path)
							}
						}
						if !reflect.DeepEqual(expected.Config, actual.Config) {
							t.Errorf("fresh/regenerated config differs: fresh=%+v regenerated=%+v", expected.Config, actual.Config)
						}
						t.FailNow()
					}
					before := acceptanceTree(t, p.dir)
					p.run(t, p.dir, true, nil, p.binary, "generate", "--fabrica-source", h.repo)
					if !reflect.DeepEqual(before, acceptanceTree(t, p.dir)) {
						t.Fatal("repeated generation changed files or manifest")
					}
					for path, enabled := range map[string]bool{"metrics_generated.go": state.Metrics, "auth_helpers_generated.go": state.Auth, "metrics_helpers_generated.go": false} {
						_, err := os.Stat(filepath.Join(p.dir, "cmd/server", path))
						if (err == nil) != enabled {
							t.Fatalf("%s enabled=%t: %v", path, enabled, err)
						}
					}
					previous = &effective
				})
			}
			// Edited tracked outputs must reject before any registration, API, or loader writes.
			path := filepath.Join(p.dir, "cmd/server/runtime_helpers_generated.go")
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				t.Fatal(err)
			}
			_, err = file.WriteString("\n// application customization\n")
			closeErr := file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			before := acceptanceTree(t, p.dir)
			out := p.run(t, p.dir, false, nil, p.binary, "generate", "--fabrica-source", h.repo)
			if !strings.Contains(string(out), "untracked or modified") {
				t.Fatalf("ownership rejection: %s", out)
			}
			if !reflect.DeepEqual(before, acceptanceTree(t, p.dir)) {
				t.Fatal("ownership rejection changed project files")
			}
		})
	}
}

const acceptanceReconciler = `// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
// SPDX-License-Identifier: MIT
package reconcilers
import("context";api "API_IMPORT")
var AcceptanceEntered=make(chan string,32)
var AcceptanceGate chan struct{}
func(r *NodeReconciler)reconcileNode(_ context.Context,n *api.Node)error{
 AcceptanceEntered<-n.GetUID()
 if AcceptanceGate!=nil{<-AcceptanceGate}
 n.Status.Ready=true;n.Status.Message="acceptance reconciled"
 return nil
}
`

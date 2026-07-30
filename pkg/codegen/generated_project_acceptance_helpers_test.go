// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const generatedProjectCommandTimeout = 2 * time.Minute

type generatedProject struct {
	root       string
	repoRoot   string
	fabricaBin string
}

type commandResult struct {
	stage  string
	stdout string
	stderr string
	err    error
}

func newGeneratedProject(t *testing.T, storageType string) generatedProject {
	t.Helper()

	root := newGeneratedProjectRoot(t)
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	project := generatedProject{
		root:       root,
		repoRoot:   repoRoot,
		fabricaBin: generatedFabricaBinary(t, repoRoot),
	}
	project.writeFixture(t, storageType, validAnnotatedTokenSource)

	return project
}

func (p generatedProject) writeFixture(t *testing.T, storageType, resourceSource string) {
	t.Helper()

	files := map[string]string{
		"go.mod": fmt.Sprintf(`module example.com/generated-annotation-acceptance

go 1.24.0

require (
	github.com/openchami/fabrica v0.0.0
	entgo.io/ent v0.14.5
)

replace github.com/openchami/fabrica => %s
`, filepath.ToSlash(p.repoRoot)),
		"apis.yaml": `groups:
  - name: acceptance.example.io
    storageVersion: v1
    versions: [v1]
    resources: [Token]
`,
		".fabrica.yaml": fmt.Sprintf(`project:
  name: generated-annotation-acceptance
  module: example.com/generated-annotation-acceptance
features:
  storage:
    enabled: true
    type: %s
    db_driver: sqlite
`, storageType),
		filepath.Join("apis", "acceptance.example.io", "v1", "token_types.go"): resourceSource,
		filepath.Join("cmd", "server", "main.go"):                              "package main\n\nfunc main() {}\n",
	}

	for path, content := range files {
		fullPath := filepath.Join(p.root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("create fixture directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}
}

func (p generatedProject) generate(t *testing.T) commandResult {
	t.Helper()

	return p.run(
		t,
		"fabrica-generate-storage",
		p.root,
		p.fabricaBin,
		"generate",
		"--storage",
		"--force",
		"--debug",
		"--fabrica-source",
		p.repoRoot,
	)
}

func (p generatedProject) tidy(t *testing.T) commandResult {
	t.Helper()

	return p.run(t, "go-mod-tidy", p.root, "go", "mod", "tidy")
}

func (p generatedProject) build(t *testing.T) commandResult {
	t.Helper()

	return p.run(t, "go-build-all", p.root, "go", "build", "./...")
}

func (p generatedProject) vet(t *testing.T) commandResult {
	t.Helper()

	return p.run(t, "go-vet-all", p.root, "go", "vet", "./...")
}

func (p generatedProject) writeResourceSource(t *testing.T, source string) {
	t.Helper()

	path := filepath.Join(p.root, "apis", "acceptance.example.io", "v1", "token_types.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("replace annotated resource source: %v", err)
	}
}

func (p generatedProject) run(
	t *testing.T,
	stage string,
	dir string,
	name string,
	args ...string,
) commandResult {
	t.Helper()
	return p.runWithTimeout(t, generatedProjectCommandTimeout, stage, dir, name, args...)
}

func (p generatedProject) runWithTimeout(
	t *testing.T,
	timeout time.Duration,
	stage string,
	dir string,
	name string,
	args ...string,
) commandResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = generatedProjectEnvironment()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		err = fmt.Errorf("%s timed out after %s: %w", stage, timeout, ctx.Err())
	}

	return commandResult{
		stage:  stage,
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

func (r commandResult) failureMessage() string {
	return fmt.Sprintf(
		"stage %q failed: %v\n--- stdout ---\n%s\n--- stderr ---\n%s",
		r.stage,
		r.err,
		r.stdout,
		r.stderr,
	)
}

const validAnnotatedTokenSource = `package v1

import (
	"context"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       TokenSpec        ` + "`json:\"spec\"`" + `
	Status     TokenStatus      ` + "`json:\"status,omitempty\"`" + `
}

type TokenSpec struct {
	// +fabrica:field:index
	// +fabrica:field:unique
	Name string ` + "`json:\"name\" validate:\"required\"`" + `

	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Value string ` + "`json:\"value\" validate:\"required\"`" + `

	// +fabrica:field:default=false
	Revoked bool ` + "`json:\"revoked\"`" + `
}

type TokenStatus struct {
	State string ` + "`json:\"state\"`" + `
}

func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string                { return "Token" }
func (r *Token) GetName() string                { return r.Metadata.Name }
func (r *Token) GetUID() string                 { return r.Metadata.UID }
func (r *Token) IsHub()                         {}
`

const malformedAnnotatedTokenSource = `package v1

import (
	"context"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       TokenSpec        ` + "`json:\"spec\"`" + `
	Status     TokenStatus      ` + "`json:\"status,omitempty\"`" + `
}

type TokenSpec struct {
	// +fabrica:field:sensitve
	Value string ` + "`json:\"value\"`" + `
}

type TokenStatus struct{}

func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string                { return "Token" }
func (r *Token) GetName() string                { return r.Metadata.Name }
func (r *Token) GetUID() string                 { return r.Metadata.UID }
func (r *Token) IsHub()                         {}
`

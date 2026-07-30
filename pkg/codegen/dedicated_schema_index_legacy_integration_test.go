//go:build integration

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeneratedDedicatedIndex_legacy_map_annotation_does_not_compile(t *testing.T) {
	root := t.TempDir()
	writeDedicatedIndexFixtureFile(t, filepath.Join(root, "go.mod"), "module example.com/invalid-index\n\ngo 1.24.0\n\nrequire entgo.io/ent v0.14.5\n")
	writeDedicatedIndexFixtureFile(t, filepath.Join(root, "invalid_test.go"), legacyMapAnnotationSource)
	runDedicatedSchemaCommand(t, root, "go", "mod", "tidy")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "test", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	if err == nil {
		t.Fatalf("legacy map annotation compiled; expected schema.Annotation interface failure\n%s", output.String())
	}
	if ctx.Err() != nil {
		t.Fatalf("legacy map annotation compile probe timed out: %v", ctx.Err())
	}
	if !strings.Contains(output.String(), "does not implement schema.Annotation") {
		t.Fatalf("compile failure does not prove the annotation contract\n%s", output.String())
	}
}

const legacyMapAnnotationSource = `package invalidindex

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/index"
)

func indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tags").Annotations(map[string]interface{}{"gin": true}),
	}
}
`

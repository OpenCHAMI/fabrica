// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	generatedEntModule  = "entgo.io/ent"
	generatedEntVersion = "v0.14.5"
)

type goModEditJSON struct {
	Require []struct {
		Path     string
		Version  string
		Indirect bool
	}
}

func TestGeneratedProjectDependencyGate_rejects_unpinned_ent(t *testing.T) {
	// Given
	root := t.TempDir()
	goMod := `module example.com/unpinned

go 1.24.0

require entgo.io/ent v0.14.4
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write unpinned go.mod: %v", err)
	}

	// When
	err := verifyPinnedEntRequirement(root)

	// Then
	if err == nil || !strings.Contains(err.Error(), "v0.14.5") {
		t.Fatalf("unpinned Ent requirement error = %v, want v0.14.5 rejection", err)
	}
}

func verifyPinnedEntRequirement(root string) error {
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("read generated go.mod: %w", err)
	}
	var document goModEditJSON
	if err := json.Unmarshal(output, &document); err != nil {
		return fmt.Errorf("decode generated go.mod: %w", err)
	}
	for _, requirement := range document.Require {
		if requirement.Path != generatedEntModule {
			continue
		}
		if requirement.Version != generatedEntVersion || requirement.Indirect {
			return fmt.Errorf("generated go.mod requires %s %s indirect=%t; want %s direct", requirement.Path, requirement.Version, requirement.Indirect, generatedEntVersion)
		}
		return nil
	}
	return fmt.Errorf("generated go.mod does not require %s %s directly", generatedEntModule, generatedEntVersion)
}

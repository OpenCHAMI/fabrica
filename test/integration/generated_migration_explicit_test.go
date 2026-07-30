// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedMigration_is_explicit_and_dedicated_only(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, dedicatedMigrationTokenSource)
	widgetPath := filepath.Join(project.root, "apis", "acceptance.example.io", "v1", "widget_types.go")
	if err := os.WriteFile(widgetPath, []byte(mixedGenericWidgetSource), 0o644); err != nil {
		t.Fatalf("write generic resource: %v", err)
	}
	apisPath := filepath.Join(project.root, "apis.yaml")
	apis := "groups:\n  - name: acceptance.example.io\n    storageVersion: v1\n    versions: [v1]\n    resources: [Token, Widget]\n"
	if err := os.WriteFile(apisPath, []byte(apis), 0o644); err != nil {
		t.Fatalf("write mixed API config: %v", err)
	}
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	// When
	declarations, calls := generatedMigrationSymbols(t, project.root)

	// Then
	if declarations["PreviewTokenMigration"] != 1 || declarations["MigrateTokenFromGeneric"] != 1 {
		t.Fatalf("dedicated migration declarations=%#v", declarations)
	}
	if declarations["PreviewWidgetMigration"] != 0 || declarations["MigrateWidgetFromGeneric"] != 0 {
		t.Fatalf("generic resource received migration declarations=%#v", declarations)
	}
	if calls["PreviewTokenMigration"] != 0 || calls["MigrateTokenFromGeneric"] != 0 {
		t.Fatalf("migration was invoked automatically: calls=%#v", calls)
	}
}

func generatedMigrationSymbols(t *testing.T, root string) (map[string]int, map[string]int) {
	t.Helper()
	declarations := make(map[string]int)
	calls := make(map[string]int)
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok {
				declarations[function.Name.Name]++
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if identifier, ok := call.Fun.(*ast.Ident); ok {
				calls[identifier.Name]++
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("inspect generated migration symbols: %v", walkErr)
	}
	return declarations, calls
}

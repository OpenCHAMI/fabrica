// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedReadHandlersEmitAndValidateETags(t *testing.T) {
	generated := generateConditionalHandlerSource(t, true)

	for _, handler := range []string{"GetNodes", "GetNode", "ListNodeVersions", "GetNodeVersion"} {
		t.Run(handler, func(t *testing.T) {
			body := generatedFunctionBody(t, generated, handler)
			assertGeneratedStatementsInOrder(t, body,
				"conditionalmiddleware.GenerateETag(",
				"conditionalmiddleware.SetETag(w, etag)",
				"conditionalmiddleware.CheckIfNoneMatch(w, r, etag)",
				"respondJSON(w, http.StatusOK,",
			)
		})
	}
}

func TestGeneratedMutationHandlersEnforceOptionalIfMatch(t *testing.T) {
	generated := generateConditionalHandlerSource(t, true)

	for _, handler := range []string{
		"UpdateNode",
		"PatchNode",
		"UpdateNodeStatus",
		"PatchNodeStatus",
		"DeleteNode",
	} {
		t.Run(handler, func(t *testing.T) {
			body := generatedFunctionBody(t, generated, handler)
			assertGeneratedStatementsInOrder(t, body,
				"storage.LoadNode(r.Context(), uid)",
				`r.Header.Get("If-Match") != ""`,
				"conditionalmiddleware.GenerateETag(",
				"conditionalmiddleware.CheckIfMatch(w, r, etag)",
			)
		})
	}

	versionDelete := generatedFunctionBody(t, generated, "DeleteNodeVersion")
	assertGeneratedStatementsInOrder(t, versionDelete,
		`r.Header.Get("If-Match") != ""`,
		"storage.GetNodeVersion(r.Context(), uid, versionID)",
		"conditionalmiddleware.GenerateETag(version)",
		"conditionalmiddleware.CheckIfMatch(w, r, etag)",
		"storage.DeleteNodeVersion(r.Context(), uid, versionID)",
	)
}

func TestGeneratedHandlersPreserveUnconditionalMode(t *testing.T) {
	generated := generateConditionalHandlerSource(t, false)

	for _, unwanted := range []string{
		`conditionalmiddleware "example.com/test/internal/middleware"`,
		"conditionalmiddleware.GenerateETag(",
		"conditionalmiddleware.CheckIfMatch(",
		"conditionalmiddleware.CheckIfNoneMatch(",
	} {
		if strings.Contains(generated, unwanted) {
			t.Errorf("conditional-disabled handlers contain %q", unwanted)
		}
	}
}

func generateConditionalHandlerSource(t *testing.T, enabled bool) string {
	t.Helper()

	outDir := t.TempDir()
	gen := NewGenerator(outDir, "main", "example.com/test")
	gen.Config.ConditionalEnabled = enabled
	gen.Config.VersioningEnabled = true
	gen.Resources = []ResourceMetadata{{
		Name:         "Node",
		PluralName:   "nodes",
		Package:      "example.com/test/apis/v1",
		PackageAlias: "v1",
		TypeName:     "*v1.Node",
		SpecType:     "v1.NodeSpec",
		StatusType:   "v1.NodeStatus",
		URLPath:      "/nodes",
		StorageName:  "Node",
		Tags:         map[string]string{"versioning": "enabled"},
	}}

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if err := gen.GenerateHandlers(); err != nil {
		t.Fatalf("GenerateHandlers: %v", err)
	}

	generated, err := os.ReadFile(filepath.Join(outDir, "node_handlers_generated.go"))
	if err != nil {
		t.Fatalf("read generated handlers: %v", err)
	}
	return string(generated)
}

func generatedFunctionBody(t *testing.T, source, name string) string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "handlers_generated.go", source, 0)
	if err != nil {
		t.Fatalf("parse generated handlers: %v", err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		return source[fset.Position(fn.Body.Pos()).Offset:fset.Position(fn.Body.End()).Offset]
	}

	t.Fatalf("generated handler %s not found", name)
	return ""
}

func assertGeneratedStatementsInOrder(t *testing.T, body string, statements ...string) {
	t.Helper()

	remaining := body
	for _, statement := range statements {
		index := strings.Index(remaining, statement)
		if index < 0 {
			t.Fatalf("generated handler missing %q:\n%s", statement, body)
		}
		remaining = remaining[index+len(statement):]
	}
}

// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

// This file implements the shared resource-import logic used by
// `fabrica add resources <path-to-structs>` and `fabrica create`.
//
// It discovers resources from a directory of hand-written struct files and, for
// each one, runs the equivalent of `fabrica add resource <Name>` and merges the
// hand-written declarations into the generated
// apis/<group>/<version>/<name>_types.go file.
//
// Technique (comment-preserving, go/ast):
//
//	go/ast is used for *discovery and positions only*. Declarations are copied
//	as verbatim byte spans taken from the original source (from the leading doc
//	comment through the closing brace), so doc comments, inline field comments,
//	block comments, struct tags, and commented-out fields are preserved exactly.
//	go/printer is deliberately not used to transplant nodes between files, which
//	would drop or misattach comments. The assembled file is finished with gofmt
//	(go/format), and imports are recomputed from actual selector usage.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	configpkg "github.com/openchami/fabrica/internal/config"
)

// resourcePart describes one resource discovered from a parts file.
type resourcePart struct {
	name      string // Fabrica resource name, e.g. RedfishEndpoint
	specName  string // Spec struct name, e.g. RedfishEndpointSpec
	partsFile string // absolute path to the hand-written struct file
}

// addResourcesFromDir discovers resources under partsDir (which must be an
// absolute path, resolved before any working-directory change) and adds and
// merges them into the fabrica project rooted at projectDir.
func addResourcesFromDir(projectDir, partsDir string) error {
	parts, err := discoverResourceParts(partsDir)
	if err != nil {
		return err
	}
	return addResourcesToProject(projectDir, parts)
}

// addResourcesToProject adds and merges every discovered resource into the
// fabrica project rooted at projectDir.
func addResourcesToProject(projectDir string, parts []resourcePart) error {
	prev, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(projectDir); err != nil {
		return fmt.Errorf("enter project dir %s: %w", projectDir, err)
	}
	defer func() { _ = os.Chdir(prev) }()

	apisConfig, err := configpkg.LoadAPIsConfig("")
	if err != nil {
		return fmt.Errorf("load apis.yaml: %w", err)
	}
	group, err := apisConfig.PrimaryGroup()
	if err != nil {
		return err
	}
	version := group.StorageVersion

	fmt.Printf("\n📦 Adding %d resource(s) from struct files...\n", len(parts))

	// Step 1: scaffold every resource (same as `fabrica add resource <Name>`).
	for _, p := range parts {
		addOpts := &addOptions{withValidation: true, withStatus: true}
		if err := runAddResource(p.name, addOpts); err != nil {
			return fmt.Errorf("add resource %s: %w", p.name, err)
		}
	}

	// Step 2: merge the hand-written declarations into each scaffold.
	for _, p := range parts {
		typesFile := filepath.Join("apis", group.Name, version, strings.ToLower(p.name)+"_types.go")
		if err := mergeResourcePart(typesFile, p); err != nil {
			return fmt.Errorf("merge %s: %w", p.name, err)
		}
	}
	return nil
}

// --- discovery --------------------------------------------------------------

// discoverResourceParts builds a resourcePart for every .go file under partsDir
// that declares a single exported <Name>Spec struct.
func discoverResourceParts(partsDir string) ([]resourcePart, error) {
	info, err := os.Stat(partsDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", partsDir)
	}

	entries, err := os.ReadDir(partsDir)
	if err != nil {
		return nil, err
	}

	var parts []resourcePart
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		path := filepath.Join(partsDir, name)

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		specName, err := findSpecStruct(file)
		if err != nil {
			// A file with no *Spec struct is a helper-only file; skip it.
			fmt.Printf("  skip %s: %v\n", name, err)
			continue
		}

		resName := titleFirst(strings.TrimSuffix(specName, "Spec"))
		parts = append(parts, resourcePart{
			name:      resName,
			specName:  specName,
			partsFile: path,
		})
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("no resource (*Spec) struct files found in %s", partsDir)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].name < parts[j].name })
	return parts, nil
}

// findSpecStruct returns the single exported <Name>Spec struct declared in file.
func findSpecStruct(file *ast.File) (string, error) {
	var found []string
	forEachTypeDecl(file, func(_ *ast.GenDecl, ts *ast.TypeSpec) {
		name := ts.Name.Name
		if _, ok := ts.Type.(*ast.StructType); !ok {
			return
		}
		if name == "Spec" || !strings.HasSuffix(name, "Spec") {
			return
		}
		if !ast.IsExported(name) {
			return
		}
		found = append(found, name)
	})

	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("no exported *Spec struct found")
	default:
		return "", fmt.Errorf("multiple *Spec structs found (%s); put one resource per file", strings.Join(found, ", "))
	}
}

// --- merge ------------------------------------------------------------------

// mergeResourcePart replaces the scaffolded Spec/Status/Validate with the
// hand-written declarations, appends helper declarations, copies resource-level
// annotations onto the envelope, fixes imports, and gofmts the result.
func mergeResourcePart(typesFile string, p resourcePart) error {
	partsSrc, err := os.ReadFile(p.partsFile)
	if err != nil {
		return err
	}
	partsFset := token.NewFileSet()
	partsFile, err := parser.ParseFile(partsFset, p.partsFile, partsSrc, parser.ParseComments)
	if err != nil {
		return err
	}

	targetSrc, err := os.ReadFile(typesFile)
	if err != nil {
		return fmt.Errorf("reading scaffolded file %s: %w", typesFile, err)
	}
	targetFset := token.NewFileSet()
	targetFile, err := parser.ParseFile(targetFset, typesFile, targetSrc, parser.ParseComments)
	if err != nil {
		return err
	}

	// Index the target's replaceable declarations.
	targetTypes := map[string]*ast.GenDecl{}
	forEachTypeDecl(targetFile, func(gd *ast.GenDecl, ts *ast.TypeSpec) {
		targetTypes[ts.Name.Name] = gd
	})
	targetFuncs := map[string]*ast.FuncDecl{}
	for _, decl := range targetFile.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			targetFuncs[funcKey(fd)] = fd
		}
	}

	var edits []edit
	var appends []string
	var added []string

	// Walk the hand-written declarations and plan edits.
	for _, decl := range partsFile.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue // imports handled separately
			}
			planGenDecl(d, partsSrc, partsFset, targetTypes, &edits, &appends, &added, targetSrc, targetFset)
		case *ast.FuncDecl:
			block := nodeSpan(partsSrc, partsFset, d.Doc, d)
			if dst, ok := targetFuncs[funcKey(d)]; ok {
				edits = append(edits, replaceEdit(targetSrc, targetFset, dst.Doc, dst, block))
			} else {
				appends = append(appends, block)
				added = append(added, funcKey(d))
			}
		}
	}

	// Copy resource-level +fabrica: directives onto the envelope declaration.
	if annots := specResourceAnnotations(partsFile, p.specName); len(annots) > 0 {
		if env, ok := targetTypes[p.name]; ok && !envelopeHasAnnotation(env, targetSrc, targetFset) {
			at := posOffset(targetFset, env.Pos()) // position of the `type` keyword
			edits = append(edits, edit{start: at, end: at, text: strings.Join(annots, "\n") + "\n"})
		}
	}

	merged := applyEdits(targetSrc, edits)
	if len(appends) > 0 {
		merged = append(bytes.TrimRight(merged, "\n"), []byte("\n\n"+strings.Join(appends, "\n\n")+"\n")...)
	}

	// Recompute imports from real usage across the merged body.
	merged, err = fixMergedImports(merged, targetFile, partsFile)
	if err != nil {
		return err
	}

	formatted, err := format.Source(merged)
	if err != nil {
		return fmt.Errorf("gofmt %s: %w", typesFile, err)
	}
	if err := os.WriteFile(typesFile, formatted, 0o644); err != nil {
		return err
	}

	fmt.Printf("  ✓ merged %s into %s", p.specName, typesFile)
	if len(added) > 0 {
		fmt.Printf(" (+ %s)", strings.Join(added, ", "))
	}
	fmt.Println()
	return nil
}

// planGenDecl plans the edit for one hand-written type/const/var declaration.
func planGenDecl(
	d *ast.GenDecl, partsSrc []byte, partsFset *token.FileSet,
	targetTypes map[string]*ast.GenDecl, edits *[]edit, appends *[]string, added *[]string,
	targetSrc []byte, targetFset *token.FileSet,
) {
	// Single-spec type declaration that matches a scaffolded type: replace it.
	if d.Tok == token.TYPE && len(d.Specs) == 1 {
		ts := d.Specs[0].(*ast.TypeSpec)
		name := ts.Name.Name
		block := nodeSpan(partsSrc, partsFset, d.Doc, d)
		if dst, ok := targetTypes[name]; ok {
			*edits = append(*edits, replaceEdit(targetSrc, targetFset, dst.Doc, dst, block))
			return
		}
		*appends = append(*appends, block)
		*added = append(*added, name)
		return
	}

	// Otherwise append the whole declaration if none of its names collide.
	for _, name := range genDeclNames(d) {
		if _, ok := targetTypes[name]; ok {
			return // a colliding name already exists; skip to avoid redefinition
		}
	}
	block := nodeSpan(partsSrc, partsFset, d.Doc, d)
	*appends = append(*appends, block)
	if names := genDeclNames(d); len(names) > 0 {
		*added = append(*added, strings.Join(names, ", "))
	}
}

// --- annotations ------------------------------------------------------------

// specResourceAnnotations returns the resource-level +fabrica: directive lines
// that appear in the <Name>Spec doc comment. Field-level (+fabrica:field:*)
// lines stay with their fields and are not returned.
func specResourceAnnotations(file *ast.File, specName string) []string {
	var lines []string
	forEachTypeDecl(file, func(gd *ast.GenDecl, ts *ast.TypeSpec) {
		if ts.Name.Name != specName {
			return
		}
		doc := gd.Doc
		if doc == nil {
			doc = ts.Doc
		}
		if doc == nil {
			return
		}
		for _, c := range doc.List {
			text := strings.TrimSpace(c.Text)
			if strings.Contains(text, "+fabrica:") && !strings.Contains(text, "+fabrica:field:") {
				lines = append(lines, c.Text)
			}
		}
	})
	return lines
}

// envelopeHasAnnotation reports whether the envelope already carries a
// +fabrica:resource directive, making injection a no-op.
func envelopeHasAnnotation(env *ast.GenDecl, src []byte, fset *token.FileSet) bool {
	if env.Doc == nil {
		return false
	}
	span := src[posOffset(fset, env.Doc.Pos()):posOffset(fset, env.Doc.End())]
	return bytes.Contains(span, []byte("+fabrica:resource"))
}

// --- imports ----------------------------------------------------------------

// importSpec is a single import's local name (alias, or "" for default) and path.
type importSpec struct{ name, path string }

// fixMergedImports rewrites the merged file's import block so it contains
// exactly the packages used by selector expressions (plus blank/dot imports),
// drawing paths from the target and parts files.
func fixMergedImports(merged []byte, targetFile, partsFile *ast.File) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", merged, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("reparse merged file: %w", err)
	}

	// Map local import name -> import spec, from both sources.
	known := map[string]importSpec{}
	var forced []importSpec // blank/dot imports are always kept
	for _, src := range []*ast.File{targetFile, partsFile} {
		for _, imp := range src.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			name := importSpecName(imp)
			if name == "_" || name == "." {
				forced = append(forced, importSpec{name, path})
				continue
			}
			known[name] = importSpec{name, path}
		}
	}

	// Collect package identifiers actually used as X in X.Sel.
	used := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})

	seen := map[string]bool{}
	var chosen []importSpec
	add := func(s importSpec) {
		key := s.name + " " + s.path
		if !seen[key] {
			seen[key] = true
			chosen = append(chosen, s)
		}
	}
	for name := range used {
		if s, ok := known[name]; ok {
			add(s)
		}
	}
	for _, s := range forced {
		add(s)
	}

	block := renderImportBlock(chosen)

	// Replace the existing import declaration, or insert after the package clause.
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			start := posOffset(fset, gd.Pos())
			if gd.Doc != nil {
				start = posOffset(fset, gd.Doc.Pos())
			}
			end := posOffset(fset, gd.End())
			return applyEdits(merged, []edit{{start: start, end: end, text: block}}), nil
		}
	}

	at := posOffset(fset, file.Name.End())
	insert := block
	if block != "" {
		insert = "\n\n" + block
	}
	return applyEdits(merged, []edit{{start: at, end: at, text: insert}}), nil
}

// renderImportBlock renders chosen imports as a sorted parenthesized block.
func renderImportBlock(specs []importSpec) string {
	if len(specs) == 0 {
		return ""
	}
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].path != specs[j].path {
			return specs[i].path < specs[j].path
		}
		return specs[i].name < specs[j].name
	})
	var b strings.Builder
	b.WriteString("import (\n")
	for _, s := range specs {
		base := s.path[strings.LastIndex(s.path, "/")+1:]
		if s.name != "" && s.name != base {
			fmt.Fprintf(&b, "\t%s %q\n", s.name, s.path)
		} else {
			fmt.Fprintf(&b, "\t%q\n", s.path)
		}
	}
	b.WriteString(")")
	return b.String()
}

func importSpecName(imp *ast.ImportSpec) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	path := strings.Trim(imp.Path.Value, `"`)
	return path[strings.LastIndex(path, "/")+1:]
}

// --- byte-span editing ------------------------------------------------------

type edit struct {
	start, end int
	text       string
}

// applyEdits applies non-overlapping edits to src, highest offset first so that
// earlier offsets stay valid.
func applyEdits(src []byte, edits []edit) []byte {
	sorted := append([]edit(nil), edits...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].start > sorted[j].start })
	out := append([]byte(nil), src...)
	for _, e := range sorted {
		out = append(out[:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	return out
}

// replaceEdit builds an edit that swaps a destination declaration (including its
// doc comment) for replacement text.
func replaceEdit(src []byte, fset *token.FileSet, doc *ast.CommentGroup, node ast.Node, replacement string) edit {
	start := posOffset(fset, node.Pos())
	if doc != nil {
		start = posOffset(fset, doc.Pos())
	}
	return edit{start: start, end: posOffset(fset, node.End()), text: replacement}
}

// nodeSpan returns the verbatim source for node, including its doc comment.
func nodeSpan(src []byte, fset *token.FileSet, doc *ast.CommentGroup, node ast.Node) string {
	start := posOffset(fset, node.Pos())
	if doc != nil {
		start = posOffset(fset, doc.Pos())
	}
	return string(src[start:posOffset(fset, node.End())])
}

func posOffset(fset *token.FileSet, pos token.Pos) int {
	return fset.Position(pos).Offset
}

// --- small AST helpers ------------------------------------------------------

func forEachTypeDecl(file *ast.File, fn func(*ast.GenDecl, *ast.TypeSpec)) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				fn(gd, ts)
			}
		}
	}
}

func genDeclNames(d *ast.GenDecl) []string {
	var names []string
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			names = append(names, s.Name.Name)
		case *ast.ValueSpec:
			for _, n := range s.Names {
				names = append(names, n.Name)
			}
		}
	}
	return names
}

// funcKey identifies a function or method by receiver type and name, so a
// hand-written method replaces the scaffolded one with the same signature slot.
func funcKey(fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		return recvTypeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
	}
	return fd.Name.Name
}

func recvTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr: // generic receiver T[P]
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	}
	return ""
}

// --- strings ----------------------------------------------------------------

func titleFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

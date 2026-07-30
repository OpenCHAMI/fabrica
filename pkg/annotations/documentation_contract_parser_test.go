// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type markdownDocument struct {
	content  string
	sections map[string]string
}

type sectionRequirement struct {
	path    string
	heading string
	claims  []string
}

var contractDocumentPaths = []string{
	"README.md",
	"CHANGELOG.md",
	"pkg/annotations/README.md",
	"pkg/annotations/doc.go",
	"docs/README.md",
	"docs/guides/middleware.md",
	"docs/guides/storage.md",
	"docs/guides/storage-ent.md",
	"examples/README.md",
	"examples/12-storage-annotations/README.md",
}

func loadDocumentationContract(t *testing.T) documentationContract {
	t.Helper()
	root := documentationContractRoot(t)
	documents := make(map[string]markdownDocument, len(contractDocumentPaths))
	for _, path := range contractDocumentPaths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read contract document %s: %v", path, err)
		}
		documents[path] = parseMarkdownDocument(string(content))
	}
	return documentationContract{documents: documents, testSymbols: parseEvidenceTestSymbols(t, repositoryRoot(t))}
}

func documentationContractRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("FABRICA_DOC_CONTRACT_ROOT"); root != "" {
		return root
	}
	return repositoryRoot(t)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate documentation contract parser")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func parseMarkdownDocument(content string) markdownDocument {
	document := markdownDocument{content: content, sections: make(map[string]string)}
	type openSection struct {
		heading string
		level   int
		body    strings.Builder
	}
	var current *openSection
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		level, heading := markdownHeading(line)
		if level > 0 {
			if current != nil {
				document.sections[current.heading] = current.body.String()
			}
			current = &openSection{heading: heading, level: level}
			continue
		}
		if current != nil {
			current.body.WriteString(line)
			current.body.WriteByte('\n')
		}
	}
	if current != nil {
		document.sections[current.heading] = current.body.String()
	}
	return document
}

func markdownHeading(line string) (int, string) {
	trimmed := strings.TrimSpace(line)
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(trimmed[level+1:])
}

func requireTableRows(document markdownDocument, heading string, expected [][]string) []error {
	section, ok := document.sections[heading]
	if !ok {
		return []error{fmt.Errorf("missing table section %q", heading)}
	}
	rows := parseMarkdownTable(section)
	if len(rows) == 0 {
		return []error{fmt.Errorf("section %q has no Markdown table", heading)}
	}
	actual := make(map[string][]string, len(rows))
	for _, row := range rows {
		actual[row[0]] = row
	}
	var errs []error
	for _, row := range expected {
		got, exists := actual[row[0]]
		if !exists {
			errs = append(errs, fmt.Errorf("section %q missing row %q", heading, row[0]))
			continue
		}
		if strings.Join(got, "\x00") != strings.Join(row, "\x00") {
			errs = append(errs, fmt.Errorf("section %q row %q = %#v, want %#v", heading, row[0], got, row))
		}
	}
	if len(actual) != len(expected) {
		errs = append(errs, fmt.Errorf("section %q has %d data rows, want %d", heading, len(actual), len(expected)))
	}
	return errs
}

func parseMarkdownTable(section string) [][]string {
	var rows [][]string
	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for index := range cells {
			cells[index] = trimCodeSpan(strings.TrimSpace(cells[index]))
		}
		if len(rows) == 0 {
			rows = append(rows, cells)
			continue
		}
		if tableSeparator(cells) {
			continue
		}
		rows = append(rows, cells)
	}
	if len(rows) > 0 {
		return rows[1:]
	}
	return nil
}

func trimCodeSpan(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, "`") && strings.HasSuffix(value, "`") {
		return value[1 : len(value)-1]
	}
	return value
}

func tableSeparator(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}

func parseEvidenceTestSymbols(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	symbols := make(map[string]struct{})
	for _, directory := range []string{"pkg/annotations", "pkg/codegen", "test/integration"} {
		entries, err := os.ReadDir(filepath.Join(root, directory))
		if err != nil {
			t.Fatalf("read test directory %s: %v", directory, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(root, directory, entry.Name()), nil, 0)
			if parseErr != nil {
				t.Fatalf("parse evidence test file %s: %v", entry.Name(), parseErr)
			}
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && function.Recv == nil && strings.HasPrefix(function.Name.Name, "Test") {
					symbols[function.Name.Name] = struct{}{}
				}
			}
		}
	}
	return symbols
}

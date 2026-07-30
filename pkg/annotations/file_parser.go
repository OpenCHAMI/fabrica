// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

// ParseFileAnnotations parses annotations from a Go source file with caching.
func ParseFileAnnotations(filename string) (map[string]*ResourceAnnotations, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		globalCache.Invalidate(filename)
		return nil, fmt.Errorf("read file: %w", err)
	}
	if cached, ok := globalCache.getResources(filename, source); ok {
		return cached, nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}
	result := make(map[string]*ResourceAnnotations)
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec, ok := specification.(*ast.TypeSpec)
			if !ok {
				continue
			}
			resourceAnnotations, parseErr := parseResourceAnnotations(typeSpec, general.Doc, fset)
			if parseErr != nil {
				return nil, parseErr
			}
			result[typeSpec.Name.Name] = resourceAnnotations
		}
	}

	globalCache.setResources(filename, source, result)
	return result, nil
}

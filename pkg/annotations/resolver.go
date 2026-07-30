// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

type typeDeclaration struct {
	spec *ast.TypeSpec
	doc  *ast.CommentGroup
}

// ResolveResourceAnnotations parses and merges a resource and its Spec declaration.
func ResolveResourceAnnotations(filename, resourceName string) (*ResourceAnnotations, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file %s: %w", filename, err)
	}

	declarations := findTypeDeclarations(file, resourceName)
	return resolveResourceDeclarations(declarations, resourceName, fset)
}

func findTypeDeclarations(file *ast.File, resourceName string) map[string]typeDeclaration {
	declarations := make(map[string]typeDeclaration, 2)
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec, ok := specification.(*ast.TypeSpec)
			if !ok {
				continue
			}
			name := typeSpec.Name.Name
			if name == resourceName || name == resourceName+"Spec" {
				declarations[name] = typeDeclaration{spec: typeSpec, doc: general.Doc}
			}
		}
	}
	return declarations
}

func resolveResourceDeclarations(
	declarations map[string]typeDeclaration,
	resourceName string,
	fset *token.FileSet,
) (*ResourceAnnotations, error) {
	result := NewResourceAnnotations()
	var err error
	if resource, exists := declarations[resourceName]; exists {
		result, err = parseResourceAnnotations(resource.spec, resource.doc, fset)
		if err != nil {
			return nil, err
		}
	}

	if spec, exists := declarations[resourceName+"Spec"]; exists {
		specAnnotations, parseErr := parseResourceAnnotations(spec.spec, spec.doc, fset)
		if parseErr != nil {
			return nil, parseErr
		}
		for fieldName, fieldAnnotations := range specAnnotations.Fields {
			if _, duplicate := result.Fields[fieldName]; duplicate {
				return nil, parseError(parseSource{
					position:  fset.Position(spec.spec.Pos()),
					typeName:  spec.spec.Name.Name,
					fieldName: fieldName,
				}, "field annotations conflict across resource and Spec declarations", nil)
			}
			result.Fields[fieldName] = fieldAnnotations
		}
	}

	return result, nil
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package catalog provides type and field metadata resolution for external imports.
//
// This package helps codegen discover field information from imported types
// to generate proper conversions between hub and spoke versions.
//
// Example:
//
//	catalog := catalog.NewCatalog()
//	err := catalog.AddModule("github.com/yourorg/netmodel", "v0.9.3")
//	if err != nil {
//	    return err
//	}
//
//	fields, err := catalog.GetFields("github.com/yourorg/netmodel/api/types", "DeviceSpec")
//	if err != nil {
//	    return err
//	}
package catalog

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Field represents metadata about a struct field
type Field struct {
	Name     string // Go field name
	Type     string // Go type
	JSONTag  string // JSON tag name
	Required bool   // Whether field is required
}

// TypeInfo represents metadata about a type
type TypeInfo struct {
	Name    string
	Package string
	Fields  []Field
}

// Catalog maintains a registry of importable types and their metadata
type Catalog struct {
	modules map[string]string    // module -> version
	types   map[string]*TypeInfo // packagePath.TypeName -> TypeInfo
}

// NewCatalog creates a new import catalog
func NewCatalog() *Catalog {
	return &Catalog{
		modules: make(map[string]string),
		types:   make(map[string]*TypeInfo),
	}
}

// AddModule registers a module with a specific version
// In a real implementation, this would fetch and cache the module
func (c *Catalog) AddModule(modulePath, version string) error {
	c.modules[modulePath] = version
	return nil
}

// GetFields returns field metadata for a type in a package
// This is a simplified implementation; production would use go/packages or similar
func (c *Catalog) GetFields(packagePath, typeName string) ([]Field, error) {
	key := packagePath + "." + typeName

	if typeInfo, ok := c.types[key]; ok {
		return typeInfo.Fields, nil
	}

	// In a production implementation, this would:
	// 1. Resolve the package using go/packages
	// 2. Parse the AST to extract field information
	// 3. Cache the results

	// For now, return an error indicating manual definition is needed
	return nil, fmt.Errorf("type %s not found in catalog; add it via apis.yaml imports", key)
}

// ScanLocalPackage scans a local package directory for type definitions
// This is used when the types are defined locally in the project
func (c *Catalog) ScanLocalPackage(pkgPath string) error {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, pkgPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse package: %w", err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				typeSpec, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					return true
				}

				// Extract fields
				var fields []Field
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						jsonTag := ""
						if field.Tag != nil {
							jsonTag = extractJSONTag(field.Tag.Value)
						}

						fields = append(fields, Field{
							Name:    name.Name,
							Type:    exprToString(field.Type),
							JSONTag: jsonTag,
						})
					}
				}

				// Store type info
				key := pkg.Name + "." + typeSpec.Name.Name
				c.types[key] = &TypeInfo{
					Name:    typeSpec.Name.Name,
					Package: pkg.Name,
					Fields:  fields,
				}

				return false
			})
		}
	}

	return nil
}

// extractJSONTag extracts the json tag value from a struct tag
func extractJSONTag(tag string) string {
	// Remove surrounding backticks
	tag = strings.Trim(tag, "`")

	// Find json:"..." part
	parts := strings.Split(tag, " ")
	for _, part := range parts {
		if strings.HasPrefix(part, "json:") {
			jsonPart := strings.TrimPrefix(part, "json:")
			jsonPart = strings.Trim(jsonPart, "\"")
			// Get just the name part (before comma)
			if idx := strings.Index(jsonPart, ","); idx != -1 {
				return jsonPart[:idx]
			}
			return jsonPart
		}
	}

	return ""
}

// exprToString converts an ast.Expr to a string representation of the type
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	default:
		return "interface{}"
	}
}

// GetTypeInfo returns full type information
func (c *Catalog) GetTypeInfo(packagePath, typeName string) (*TypeInfo, error) {
	key := packagePath + "." + typeName

	if typeInfo, ok := c.types[key]; ok {
		return typeInfo, nil
	}

	return nil, fmt.Errorf("type %s not found", key)
}

// LoadFromDirectory loads all Go types from a directory
func (c *Catalog) LoadFromDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Parse each Go file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files that don't parse
		}

		// Extract type definitions
		ast.Inspect(node, func(n ast.Node) bool {
			typeSpec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			// Extract fields
			var fields []Field
			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					jsonTag := ""
					if field.Tag != nil {
						jsonTag = extractJSONTag(field.Tag.Value)
					}

					fields = append(fields, Field{
						Name:    name.Name,
						Type:    exprToString(field.Type),
						JSONTag: jsonTag,
					})
				}
			}

			// Store type info
			key := node.Name.Name + "." + typeSpec.Name.Name
			c.types[key] = &TypeInfo{
				Name:    typeSpec.Name.Name,
				Package: node.Name.Name,
				Fields:  fields,
			}

			return false
		})

		return nil
	})
}

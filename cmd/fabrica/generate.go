// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexlovelltroy/fabrica/pkg/codegen"
	"github.com/spf13/cobra"
)

func newGenerateCommand() *cobra.Command {
	var (
		handlers bool
		storage  bool
		client   bool
		openapi  bool
		all      bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate code from resource definitions",
		Long: `Generate server handlers, storage adapters, client code, and OpenAPI specs
from your resource definitions.

Examples:
  fabrica generate                    # Generate all
  fabrica generate --handlers         # Just handlers
  fabrica generate --client --openapi # Client + OpenAPI
`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !handlers && !storage && !client && !openapi {
				all = true
			}

			fmt.Println("🔧 Generating code...")

			// Read go.mod to get module path
			modulePath, err := getModulePath()
			if err != nil {
				return fmt.Errorf("failed to read module path: %w (make sure you're in a Go module)", err)
			}

			// Discover resources in pkg/resources
			resources, err := discoverResources()
			if err != nil {
				return fmt.Errorf("failed to discover resources: %w", err)
			}

			if len(resources) == 0 {
				fmt.Println("⚠️  No resources found in pkg/resources/")
				fmt.Println("   Run 'fabrica add resource <name>' to create a resource first")
				return nil
			}

			fmt.Printf("📦 Found %d resource(s): %s\n", len(resources), strings.Join(resources, ", "))

			// Create generator for server code
			if all || handlers || storage || openapi {
				gen := codegen.NewGenerator("cmd/server", "main", modulePath)

				// Register resources (this is simplified - in real implementation we'd load actual types)
				for _, resourceName := range resources {
					fmt.Printf("  ├─ Registering %s...\n", resourceName)
					// Note: This is a placeholder - actual implementation would need to load and register actual types
				}

				if err := gen.LoadTemplates(); err != nil {
					return fmt.Errorf("failed to load templates: %w", err)
				}

				if all || handlers {
					fmt.Println("  ├─ Generating handlers...")
					if err := gen.GenerateHandlers(); err != nil {
						return fmt.Errorf("failed to generate handlers: %w", err)
					}
				}

				if all || storage {
					fmt.Println("  ├─ Generating storage...")
					if err := gen.GenerateStorage(); err != nil {
						return fmt.Errorf("failed to generate storage: %w", err)
					}
				}

				if all || openapi {
					fmt.Println("  ├─ Generating OpenAPI spec...")
					if err := gen.GenerateOpenAPI(); err != nil {
						return fmt.Errorf("failed to generate OpenAPI spec: %w", err)
					}
				}
			}

			// Generate client code
			if all || client {
				gen := codegen.NewGenerator("pkg/client", "client", modulePath)

				if err := gen.LoadTemplates(); err != nil {
					return fmt.Errorf("failed to load templates: %w", err)
				}

				fmt.Println("  ├─ Generating client code...")
				if err := gen.GenerateClient(); err != nil {
					return fmt.Errorf("failed to generate client: %w", err)
				}
			}

			fmt.Println("  └─ Done!")
			fmt.Println()
			fmt.Println("✅ Code generation complete!")

			return nil
		},
	}

	cmd.Flags().BoolVar(&handlers, "handlers", false, "Generate HTTP handlers")
	cmd.Flags().BoolVar(&storage, "storage", false, "Generate storage adapters")
	cmd.Flags().BoolVar(&client, "client", false, "Generate client code")
	cmd.Flags().BoolVar(&openapi, "openapi", false, "Generate OpenAPI spec")

	return cmd
}

// getModulePath reads the module path from go.mod
func getModulePath() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// discoverResources scans pkg/resources for resource definitions
func discoverResources() ([]string, error) {
	resourcesDir := "pkg/resources"

	if _, err := os.Stat(resourcesDir); os.IsNotExist(err) {
		return nil, nil // No resources directory yet
	}

	var resources []string

	err := filepath.Walk(resourcesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Parse the file to find resource type definitions
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files that don't parse
		}

		// Look for struct types that embed resource.Resource
		ast.Inspect(node, func(n ast.Node) bool {
			typeSpec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			// Check if it embeds resource.Resource
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 { // Embedded field
					if sel, ok := field.Type.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok {
							if ident.Name == "resource" && sel.Sel.Name == "Resource" {
								resources = append(resources, typeSpec.Name.Name)
								return false
							}
						}
					}
				}
			}

			return true
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return resources, nil
}

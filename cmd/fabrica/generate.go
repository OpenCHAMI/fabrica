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
	"os/exec"
	"path/filepath"
	"strings"

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

			// Check if registration file exists or needs regenerating
			regFile := "pkg/resources/register_generated.go"
			needsRegistration := false
			if _, err := os.Stat(regFile); os.IsNotExist(err) {
				needsRegistration = true
			}

			// Auto-generate registration file if missing
			if needsRegistration {
				fmt.Println()
				fmt.Println("📝 Registration file not found, creating it...")
				if err := generateRegistrationFile(); err != nil {
					return fmt.Errorf("failed to generate registration file: %w", err)
				}
				fmt.Println()
			}

			// Ensure dependencies are available by running go mod tidy
			fmt.Println("📥 Ensuring dependencies are available...")
			tidyCmd := exec.Command("go", "mod", "tidy")
			tidyCmd.Stdout = nil // Suppress output unless there's an error
			tidyCmd.Stderr = nil
			if err := tidyCmd.Run(); err != nil {
				fmt.Println("⚠️  Warning: go mod tidy failed, continuing anyway...")
			}

			// Check if authorization is enabled (policies directory exists)
			authEnabled := false
			if _, err := os.Stat("policies"); err == nil {
				authEnabled = true
			}

			// Generate server code (handlers, storage, openapi)
			if all || handlers || storage || openapi {
				if err := generateCodeWithRunner(modulePath, "cmd/server", "main", all || handlers, all || storage, all || openapi, false, authEnabled); err != nil {
					return fmt.Errorf("failed to generate server code: %w", err)
				}
			}

			// Generate client code
			if all || client {
				if err := generateCodeWithRunner(modulePath, "pkg/client", "client", false, false, false, true, false); err != nil {
					return fmt.Errorf("failed to generate client code: %w", err)
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
		moduleName, found := strings.CutPrefix(line, "module ")
		if found {
			return strings.TrimSpace(moduleName), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// generateCodeWithRunner creates and runs a temporary codegen program
func generateCodeWithRunner(modulePath, outputDir, packageName string, handlers, storage, openapi, client, authEnabled bool) error {
	// Create runner in the project's cmd directory to have access to go.mod
	runnerDir := filepath.Join("cmd", ".fabrica-codegen")
	if err := os.MkdirAll(runnerDir, 0755); err != nil {
		return fmt.Errorf("failed to create runner directory: %w", err)
	}
	defer os.RemoveAll(runnerDir) // nolint:errcheck

	// Generate the runner program
	runnerCode := generateRunnerCode(modulePath, outputDir, packageName, handlers, storage, openapi, client, authEnabled)

	runnerPath := filepath.Join(runnerDir, "main.go")
	if err := os.WriteFile(runnerPath, []byte(runnerCode), 0644); err != nil {
		return fmt.Errorf("failed to write runner: %w", err)
	}

	// Run the codegen runner from the project root
	cmd := exec.Command("go", "run", runnerPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "." // Run in project root

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	return nil
}

// generateRunnerCode creates the source code for the temporary codegen runner
func generateRunnerCode(modulePath, outputDir, packageName string, handlers, storage, openapi, client, authEnabled bool) string {
	var generationCalls strings.Builder

	if packageName == "main" {
		// Server-side generation
		generationCalls.WriteString("\tif err := gen.LoadTemplates(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to load templates: %v\", err)\n")
		generationCalls.WriteString("\t}\n\n")

		// Enable auth for all resources if auth is enabled
		if authEnabled {
			generationCalls.WriteString("\t// Enable authorization for all resources\n")
			generationCalls.WriteString("\tfor _, res := range gen.Resources {\n")
			generationCalls.WriteString("\t\tgen.EnableAuthForResource(res.Name)\n")
			generationCalls.WriteString("\t}\n\n")
		}

		if handlers {
			generationCalls.WriteString("\tif err := gen.GenerateHandlers(); err != nil {\n")
			generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate handlers: %v\", err)\n")
			generationCalls.WriteString("\t}\n")
		}

		if storage {
			generationCalls.WriteString("\tif err := gen.GenerateStorage(); err != nil {\n")
			generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate storage: %v\", err)\n")
			generationCalls.WriteString("\t}\n")
		}

		if openapi {
			generationCalls.WriteString("\tif err := gen.GenerateOpenAPI(); err != nil {\n")
			generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate OpenAPI: %v\", err)\n")
			generationCalls.WriteString("\t}\n")
		}

		// Always generate routes and models if doing server-side generation
		generationCalls.WriteString("\tif err := gen.GenerateRoutes(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate routes: %v\", err)\n")
		generationCalls.WriteString("\t}\n")

		generationCalls.WriteString("\tif err := gen.GenerateModels(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate models: %v\", err)\n")
		generationCalls.WriteString("\t}\n")
	} else if client {
		// Client-side generation
		generationCalls.WriteString("\tif err := gen.LoadTemplates(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to load templates: %v\", err)\n")
		generationCalls.WriteString("\t}\n\n")

		generationCalls.WriteString("\tif err := gen.GenerateClient(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate client: %v\", err)\n")
		generationCalls.WriteString("\t}\n")

		generationCalls.WriteString("\tif err := gen.GenerateClientModels(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate client models: %v\", err)\n")
		generationCalls.WriteString("\t}\n")
	}

	return fmt.Sprintf(`package main

import (
	"log"

	"github.com/alexlovelltroy/fabrica/pkg/codegen"
	"%s/pkg/resources"
)

func main() {
	gen := codegen.NewGenerator("%s", "%s", "%s")

	if err := resources.RegisterAllResources(gen); err != nil {
		log.Fatalf("Failed to register resources: %%v", err)
	}

%s}
`, modulePath, outputDir, packageName, modulePath, generationCalls.String())
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

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

// FabricaConfig represents the .fabrica.yaml configuration file
type FabricaConfig struct {
	Project struct {
		ModulePath  string `yaml:"module_path"`
		Description string `yaml:"description"`
	} `yaml:"project"`
	Features struct {
		Auth      bool `yaml:"auth"`
		Storage   bool `yaml:"storage"`
		HSM       bool `yaml:"hsm"`
		LegacyAPI bool `yaml:"legacy_api"`
		Metrics   bool `yaml:"metrics"`
		Version   bool `yaml:"version"`
	} `yaml:"features"`
	Storage struct {
		Type     string `yaml:"type"`
		DBDriver string `yaml:"db_driver"`
	} `yaml:"storage"`
}

// readFabricaConfig reads the .fabrica.yaml configuration file
func readFabricaConfig() (*FabricaConfig, error) {
	configPath := ".fabrica.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil // No config file found, not an error
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .fabrica.yaml: %w", err)
	}

	var config FabricaConfig

	// Simple parser for our specific YAML format
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Look for storage type
		if strings.Contains(line, "type:") && strings.Contains(line, "ent") {
			config.Storage.Type = "ent"
		} else if strings.Contains(line, "type:") && strings.Contains(line, "file") {
			config.Storage.Type = "file"
		}

		// Look for db_driver
		if strings.Contains(line, "db_driver:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				driver := strings.TrimSpace(parts[1])
				driver = strings.Split(driver, "#")[0] // Remove comments
				driver = strings.TrimSpace(driver)
				config.Storage.DBDriver = driver
			}
		}
	}

	return &config, nil
}

func newGenerateCommand() *cobra.Command {
	var (
		handlers bool
		storage  bool
		client   bool
		openapi  bool
		all      bool
		debug    bool
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
			if debug {
				fmt.Println("🔍 Reading go.mod...")
			}
			modulePath, err := getModulePath()
			if err != nil {
				return fmt.Errorf("failed to read module path: %w (make sure you're in a Go module)", err)
			}
			if debug {
				fmt.Printf("  Module: %s\n", modulePath)
			}

			// Discover resources in pkg/resources
			if debug {
				fmt.Println("🔍 Discovering resources in pkg/resources/...")
			}
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
				if err := generateRegistrationFile(debug); err != nil {
					return fmt.Errorf("failed to generate registration file: %w", err)
				}
				fmt.Println()
			} else if debug {
				fmt.Printf("📝 Registration file exists: %s\n", regFile)
			}

			// Ensure dependencies are available by running go mod tidy
			if debug {
				fmt.Println("📥 Running go mod tidy...")
			} else {
				fmt.Println("📥 Ensuring dependencies are available...")
			}
			tidyCmd := exec.Command("go", "mod", "tidy")
			if debug {
				tidyCmd.Stdout = os.Stdout
				tidyCmd.Stderr = os.Stderr
			} else {
				tidyCmd.Stdout = nil // Suppress output unless there's an error
				tidyCmd.Stderr = nil
			}
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
				if debug {
					fmt.Println("📦 Generating server code...")
				}
				if err := generateCodeWithRunner(modulePath, "cmd/server", "main", all || handlers, all || storage, all || openapi, false, authEnabled, debug); err != nil {
					return fmt.Errorf("failed to generate server code: %w", err)
				}
			}

			// Generate client code
			if all || client {
				fmt.Println("📦 Generating client code...")
				if err := generateCodeWithRunner(modulePath, "pkg/client", "client", false, false, false, true, false, debug); err != nil {
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
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output showing detailed generation steps")

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

// detectStorageType detects the storage type from the project structure
func detectStorageType() string {
	// Check if the main.go file contains Ent imports (even if commented)
	if data, err := os.ReadFile("cmd/server/main.go"); err == nil {
		content := string(data)
		if strings.Contains(content, "internal/storage/ent") ||
			strings.Contains(content, "github.com/mattn/go-sqlite3") ||
			strings.Contains(content, "_\"github.com/mattn/go-sqlite3\"") {
			return "ent"
		}
	}

	// Default to file storage
	return "file"
}

// generateCodeWithRunner creates and runs a temporary codegen program
func generateCodeWithRunner(modulePath, outputDir, packageName string, handlers, storage, openapi, client, authEnabled, debug bool) error {
	// Create output directory if it doesn't exist
	if debug {
		fmt.Printf("  Creating output directory: %s\n", outputDir)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create runner in the project's cmd directory to have access to go.mod
	runnerDir := filepath.Join("cmd", ".fabrica-codegen")
	if err := os.MkdirAll(runnerDir, 0755); err != nil {
		return fmt.Errorf("failed to create runner directory: %w", err)
	}
	defer os.RemoveAll(runnerDir) // nolint:errcheck

	// Generate the runner program
	if debug {
		fmt.Println("  Generating codegen runner...")
	}

	// Detect storage type before generating runner
	storageType := detectStorageType()
	if debug {
		fmt.Printf("  Detected storage type: %s\n", storageType)
	}

	runnerCode := generateRunnerCode(modulePath, outputDir, packageName, handlers, storage, openapi, client, authEnabled, debug, storageType)

	runnerPath := filepath.Join(runnerDir, "main.go")
	if err := os.WriteFile(runnerPath, []byte(runnerCode), 0644); err != nil {
		return fmt.Errorf("failed to write runner: %w", err)
	}

	// Run the codegen runner from the project root
	if debug {
		fmt.Println("  Running code generator...")
	}
	// Use relative path starting with ./ so go run uses the project's go.mod and replace directives
	cmd := exec.Command("go", "run", "./"+runnerDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "." // Run in project root

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	return nil
}

// generateRunnerCode creates the source code for the temporary codegen runner
func generateRunnerCode(modulePath, outputDir, packageName string, handlers, storage, openapi, client, authEnabled, debug bool, storageType string) string {
	var generationCalls strings.Builder

	if packageName == "main" {
		// Server-side generation
		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Loading templates...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.LoadTemplates(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to load templates: %v\", err)\n")
		generationCalls.WriteString("\t}\n")

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
		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Loading templates...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.LoadTemplates(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to load templates: %v\", err)\n")
		generationCalls.WriteString("\t}\n\n")

		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Generating client library...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.GenerateClient(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate client: %v\", err)\n")
		generationCalls.WriteString("\t}\n")

		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Generating client models...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.GenerateClientModels(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate client models: %v\", err)\n")
		generationCalls.WriteString("\t}\n")

		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Generating client CLI...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.GenerateClientCmd(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate client CLI: %v\", err)\n")
		generationCalls.WriteString("\t}\n")
	}

	verboseFlag := "false"
	fmtImport := ""
	if debug {
		verboseFlag = "true"
		fmtImport = "\t\"fmt\"\n"
	}

	return fmt.Sprintf(`package main

import (
%s	"log"

	"github.com/alexlovelltroy/fabrica/pkg/codegen"
	"%s/pkg/resources"
)

func main() {
	gen := codegen.NewGenerator("%s", "%s", "%s")
	gen.Verbose = %s
	
	// Configure storage type - passed from main generate command
	gen.SetStorageType("%s")
	if "%s" == "ent" {
		gen.SetDBDriver("sqlite") // Default to sqlite for now
	}

	if err := resources.RegisterAllResources(gen); err != nil {
		log.Fatalf("Failed to register resources: %%v", err)
	}

%s}
`, fmtImport, modulePath, outputDir, packageName, modulePath, verboseFlag, storageType, storageType, generationCalls.String())
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

// generateRegistrationFile creates pkg/resources/register_generated.go
func generateRegistrationFile(debug bool) error {
	if !debug {
		fmt.Println("🔍 Discovering resources...")
	}

	// 1. Read go.mod to get module path
	modulePath, err := getModulePath()
	if err != nil {
		return fmt.Errorf("failed to get module path: %w (make sure you're in a Go module)", err)
	}

	// 2. Discover resources
	resources, err := discoverResources()
	if err != nil {
		return fmt.Errorf("failed to discover resources: %w", err)
	}

	if len(resources) == 0 {
		fmt.Println("⚠️  No resources found in pkg/resources/")
		fmt.Println("   Run 'fabrica add resource <name>' to create a resource first")
		return nil
	}

	if !debug {
		fmt.Printf("📦 Found %d resource(s): %s\n", len(resources), strings.Join(resources, ", "))
	}

	// 3. Generate registration file
	content := generateRegistrationCode(modulePath, resources)

	// 4. Ensure pkg/resources directory exists
	resourcesDir := filepath.Join("pkg", "resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return fmt.Errorf("failed to create resources directory: %w", err)
	}

	// 5. Write to pkg/resources/register_generated.go
	outputPath := filepath.Join(resourcesDir, "register_generated.go")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write registration file: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Generated %s\n", outputPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  fabrica generate                # Generate handlers and storage")
	fmt.Println("  go run cmd/server/main.go       # Start the server")

	return nil
}

// generateRegistrationCode creates the content of the registration file
func generateRegistrationCode(modulePath string, resources []string) string {
	var imports strings.Builder
	var registrations strings.Builder

	for _, resource := range resources {
		pkg := strings.ToLower(resource)
		imports.WriteString(fmt.Sprintf("\t\"%s/pkg/resources/%s\"\n", modulePath, pkg))
		registrations.WriteString(fmt.Sprintf("\tif err := gen.RegisterResource(&%s.%s{}); err != nil {\n", pkg, resource))
		registrations.WriteString(fmt.Sprintf("\t\treturn fmt.Errorf(\"failed to register %s: %%w\", err)\n", resource))
		registrations.WriteString("\t}\n")
	}

	return fmt.Sprintf(`// Code generated by fabrica codegen init. DO NOT EDIT.
package resources

import (
	"fmt"

	"github.com/alexlovelltroy/fabrica/pkg/codegen"
%s)

// RegisterAllResources registers all discovered resources with the generator.
// This file is auto-generated. Re-run 'fabrica codegen init' after adding resources.
func RegisterAllResources(gen *codegen.Generator) error {
%s
	return nil
}
`, imports.String(), registrations.String())
}

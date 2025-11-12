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
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// readFabricaConfig reads the .fabrica.yaml configuration file
// Now uses the comprehensive config system from config.go
func readFabricaConfig() (*FabricaConfig, error) {
	// Try to load config from current directory
	config, err := LoadConfig("")
	if err != nil {
		// If file doesn't exist, return nil without error (optional config)
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return config, nil
}

func newGenerateCommand() *cobra.Command {
	var (
		handlers bool
		storage  bool
		client   bool
		openapi  bool
		all      bool
		debug    bool
		force    bool
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

			// Check version compatibility before regenerating (only if generated code exists)
			// Skip version check for fresh projects (where registration file doesn't exist)
			if !needsRegistration {
				generatedVersion := detectGeneratedVersion()
				if generatedVersion != "" && debug {
					fmt.Printf("🔍 Detected generated code version: %s\n", generatedVersion)
				}

				canProceed, err := checkVersionCompatibility(version, generatedVersion, force)
				if err != nil {
					return fmt.Errorf("version check failed: %w", err)
				}
				if !canProceed {
					return fmt.Errorf("regeneration blocked due to version incompatibility (use --force to override)")
				}
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

			// Note: We don't run go mod tidy here because:
			// 1. Generated code may introduce new imports
			// 2. The user should run it after generation completes
			// This avoids circular dependency issues with code generators like Ent

			// Generate server code (handlers, storage, openapi)
			if all || handlers || storage || openapi {
				if debug {
					fmt.Println("📦 Generating server code...")
				}
				if err := generateCodeWithRunner(modulePath, "cmd/server", "main", all || handlers, all || storage, all || openapi, false, debug); err != nil {
					return fmt.Errorf("failed to generate server code: %w", err)
				}
			}

			// Generate client code
			if all || client {
				fmt.Println("📦 Generating client code...")
				if err := generateCodeWithRunner(modulePath, "pkg/client", "client", false, false, false, true, debug); err != nil {
					return fmt.Errorf("failed to generate client code: %w", err)
				}
			}

			// Check if reconciliation is enabled in config
			config, err := readFabricaConfig()
			if err == nil && config != nil && config.Features.Reconciliation.Enabled {
				fmt.Println("🔄 Generating reconciliation code...")
				if err := generateCodeWithRunner(modulePath, "pkg/reconcilers", "reconcile", false, false, false, false, debug); err != nil {
					return fmt.Errorf("failed to generate reconciliation code: %w", err)
				}
			}

			// Auto-generate Ent client code if using Ent storage
			storageType := detectStorageType()
			if storageType == "ent" && (all || storage) {
				fmt.Println("🔄 Generating Ent client code...")

				if err := generateEntCode(debug); err != nil {
					return fmt.Errorf("failed to generate ent code: %w", err)
				}

				if debug {
					fmt.Println("  ✅ Ent client code generated")
				}
			}

			fmt.Println("  └─ Done!")
			fmt.Println()
			fmt.Println("✅ Code generation complete!")
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  go mod tidy                     # Update dependencies")
			fmt.Println("  go run ./cmd/server       # Start the server")

			return nil
		},
	}

	cmd.Flags().BoolVar(&handlers, "handlers", false, "Generate HTTP handlers")
	cmd.Flags().BoolVar(&storage, "storage", false, "Generate storage adapters")
	cmd.Flags().BoolVar(&client, "client", false, "Generate client code")
	cmd.Flags().BoolVar(&openapi, "openapi", false, "Generate OpenAPI spec")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output showing detailed generation steps")
	cmd.Flags().BoolVar(&force, "force", false, "Force regeneration even with version warnings")

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

// detectStorageType detects the storage type from the project configuration
func detectStorageType() string {
	// First, check .fabrica.yaml configuration
	if config, err := readFabricaConfig(); err == nil && config != nil {
		switch config.Features.Storage.Type {
		case "ent":
			return "ent"
		case "file":
			return "file"
		}
	}

	// Fallback: Check if the main.go file contains Ent imports (even if commented)
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
func generateCodeWithRunner(modulePath, outputDir, packageName string, handlers, storage, openapi, client, debug bool) error {
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

	runnerCode := generateRunnerCode(modulePath, outputDir, packageName, handlers, storage, openapi, client, debug, storageType)

	runnerPath := filepath.Join(runnerDir, "main.go")
	if err := os.WriteFile(runnerPath, []byte(runnerCode), 0644); err != nil {
		return fmt.Errorf("failed to write runner: %w", err)
	}

	// Run the codegen runner from the project root
	if debug {
		fmt.Println("  Running code generator...")
	}
	// Use relative path starting with ./ so go run uses the project's go.mod and replace directives
	// Use -mod=mod to allow go.mod updates during code generation (needed for Ent and other generators)
	cmd := exec.Command("go", "run", "-mod=mod", "./"+runnerDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "." // Run in project root

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	return nil
}

// generateRunnerCode creates the source code for the temporary codegen runner
func generateRunnerCode(modulePath, outputDir, packageName string, handlers, storage, openapi, client, debug bool, storageType string) string {
	var generationCalls strings.Builder

	if packageName == "main" {
		// Server-side generation
		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Loading templates...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.LoadTemplates(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to load templates: %v\", err)\n")
		generationCalls.WriteString("\t}\n")

		if handlers {
			generationCalls.WriteString("\tif err := gen.GenerateHandlers(); err != nil {\n")
			generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate handlers: %v\", err)\n")
			generationCalls.WriteString("\t}\n")
			// Always generate middleware when generating handlers
			generationCalls.WriteString("\tif err := gen.GenerateMiddleware(); err != nil {\n")
			generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate middleware: %v\", err)\n")
			generationCalls.WriteString("\t}\n")
		}

		if storage {
			// Generate Ent schemas first if using Ent storage
			generationCalls.WriteString("\t// Generate Ent schemas if using Ent storage\n")
			generationCalls.WriteString("\tif err := gen.GenerateEntSchemas(); err != nil {\n")
			generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate Ent schemas: %v\", err)\n")
			generationCalls.WriteString("\t}\n")
			generationCalls.WriteString("\tif err := gen.GenerateEntAdapter(); err != nil {\n")
			generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate Ent adapter: %v\", err)\n")
			generationCalls.WriteString("\t}\n")
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
	} else if packageName == "reconcile" {
		// Reconciliation code generation
		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Loading templates...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.LoadTemplates(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to load templates: %v\", err)\n")
		generationCalls.WriteString("\t}\n\n")

		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Generating reconcilers...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.GenerateReconcilers(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate reconcilers: %v\", err)\n")
		generationCalls.WriteString("\t}\n\n")

		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Generating reconciler registration...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.GenerateReconcilerRegistration(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate reconciler registration: %v\", err)\n")
		generationCalls.WriteString("\t}\n\n")

		if debug {
			generationCalls.WriteString("\tfmt.Println(\"  Generating event handlers...\")\n")
		}
		generationCalls.WriteString("\tif err := gen.GenerateEventHandlers(); err != nil {\n")
		generationCalls.WriteString("\t\tlog.Fatalf(\"Failed to generate event handlers: %v\", err)\n")
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
	"os"

	"github.com/openchami/fabrica/pkg/codegen"
	"%s/pkg/resources"
	"gopkg.in/yaml.v3"
)

// FabricaConfig structures to load .fabrica.yaml
type FabricaConfig struct {
	Features FeaturesConfig `+"`yaml:\"features\"`"+`
}

type FeaturesConfig struct {
	Validation  ValidationConfig  `+"`yaml:\"validation\"`"+`
	Conditional ConditionalConfig `+"`yaml:\"conditional\"`"+`
	Versioning  VersioningConfig  `+"`yaml:\"versioning\"`"+`
	Events      EventsConfig      `+"`yaml:\"events\"`"+`
	Storage     StorageConfig     `+"`yaml:\"storage\"`"+`
}

type ValidationConfig struct {
	Enabled bool   `+"`yaml:\"enabled\"`"+`
	Mode    string `+"`yaml:\"mode\"`"+`
}

type ConditionalConfig struct {
	Enabled       bool   `+"`yaml:\"enabled\"`"+`
	ETagAlgorithm string `+"`yaml:\"etag_algorithm\"`"+`
}

type EventsConfig struct {
	Enabled bool   `+"`yaml:\"enabled\"`"+`
	BusType string `+"`yaml:\"bus_type\"`"+`
}

type VersioningConfig struct {
	Enabled  bool   `+"`yaml:\"enabled\"`"+`
	Strategy string `+"`yaml:\"strategy\"`"+`
}

type StorageConfig struct {
	Type     string `+"`yaml:\"type\"`"+`
	DBDriver string `+"`yaml:\"db_driver\"`"+`
}

func loadConfig() (*FabricaConfig, error) {
	data, err := os.ReadFile(".fabrica.yaml")
	if err != nil {
		return nil, err
	}

	var config FabricaConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	gen := codegen.NewGenerator("%s", "%s", "%s")
	gen.Verbose = %s
	gen.Version = "%s" // Fabrica version used for generation

	// Configure storage type - passed from main generate command
	gen.SetStorageType("%s")
	if "%s" == "ent" {
		gen.SetDBDriver("sqlite") // Default to sqlite for now
	}

	// Load .fabrica.yaml and apply configuration to generator
	if config, err := loadConfig(); err == nil {
		// Update generator config from .fabrica.yaml
		gen.Config.ValidationEnabled = config.Features.Validation.Enabled
		gen.Config.ValidationMode = config.Features.Validation.Mode
		gen.Config.ConditionalEnabled = config.Features.Conditional.Enabled
		gen.Config.ETagAlgorithm = config.Features.Conditional.ETagAlgorithm
		gen.Config.VersioningEnabled = config.Features.Versioning.Enabled
		gen.Config.VersionStrategy = config.Features.Versioning.Strategy
		gen.Config.EventsEnabled = config.Features.Events.Enabled
		gen.Config.EventBusType = config.Features.Events.BusType

		// Override storage config from .fabrica.yaml if present
		if config.Features.Storage.Type != "" {
			gen.SetStorageType(config.Features.Storage.Type)
			gen.Config.StorageType = config.Features.Storage.Type
		}
		if config.Features.Storage.DBDriver != "" {
			gen.SetDBDriver(config.Features.Storage.DBDriver)
			gen.Config.DBDriver = config.Features.Storage.DBDriver
		}
	}

	if err := resources.RegisterAllResources(gen); err != nil {
		log.Fatalf("Failed to register resources: %%v", err)
	}

%s}
`, fmtImport, modulePath, outputDir, packageName, modulePath, verboseFlag, version, storageType, storageType, generationCalls.String())
}

// discoverResources scans for resource definitions in both legacy and versioned modes
func discoverResources() ([]string, error) {
	// Load config to determine mode
	config, err := readFabricaConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Check if versioning is enabled
	if config != nil && config.Features.Versioning.Enabled && len(config.Features.Versioning.Versions) > 0 {
		// Versioned mode: scan apis/<group>/<storage-version>/
		return discoverVersionedResources(config)
	}

	// Legacy mode: scan pkg/resources/
	return discoverLegacyResources()
}

// discoverVersionedResources scans apis/<group>/<storage-version>/ for resource definitions
func discoverVersionedResources(config *FabricaConfig) ([]string, error) {
	// Use storage version (hub) for generation
	hubDir := filepath.Join("apis", config.Features.Versioning.Group, config.Features.Versioning.StorageVersion)

	if _, err := os.Stat(hubDir); os.IsNotExist(err) {
		// Hub directory doesn't exist yet, return resources from config
		return config.Features.Versioning.Resources, nil
	}

	var resources []string

	err := filepath.Walk(hubDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files and non-type files
		if info.IsDir() || !strings.HasSuffix(path, "_types.go") {
			return nil
		}

		// Parse the file to find resource type definitions
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files that don't parse
		}

		// Look for struct types with APIVersion, Kind, Metadata fields (flattened envelope)
		ast.Inspect(node, func(n ast.Node) bool {
			typeSpec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			// Check if it has flattened envelope fields
			hasAPIVersion := false
			hasKind := false
			hasMetadata := false

			for _, field := range structType.Fields.List {
				if len(field.Names) > 0 {
					fieldName := field.Names[0].Name
					switch fieldName {
					case "APIVersion":
						hasAPIVersion = true
					case "Kind":
						hasKind = true
					case "Metadata":
						hasMetadata = true
					}
				}
			}

			// If it has all three flattened envelope fields, it's a resource
			if hasAPIVersion && hasKind && hasMetadata {
				resources = append(resources, typeSpec.Name.Name)
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

// discoverLegacyResources scans pkg/resources for resource definitions (legacy mode)
func discoverLegacyResources() ([]string, error) {
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

		// Parse the file to find resource type definitions and markers
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

	// Load config to determine mode
	config, err := readFabricaConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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
		if config != nil && config.Features.Versioning.Enabled {
			fmt.Printf("⚠️  No resources found in apis/%s/%s/\n", config.Features.Versioning.Group, config.Features.Versioning.StorageVersion)
		} else {
			fmt.Println("⚠️  No resources found in pkg/resources/")
		}
		fmt.Println("   Run 'fabrica add resource <name>' to create a resource first")
		return nil
	}

	if !debug {
		fmt.Printf("📦 Found %d resource(s): %s\n", len(resources), strings.Join(resources, ", "))
	}

	// 3. Generate registration file
	var content string
	if config != nil && config.Features.Versioning.Enabled {
		content = generateVersionedRegistrationCode(modulePath, config, resources)
	} else {
		content = generateRegistrationCode(modulePath, resources)
	}

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
	fmt.Println("  go mod tidy                     # Update dependencies")
	fmt.Println("  go run ./cmd/server/       # Start the server")

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

		// After registration, set per-resource tags if markers are present.
		// Marker: // +fabrica:resource-versioning=enabled on the resource source file
		registrations.WriteString("\t// Set per-resource tags based on source markers\n")
		registrations.WriteString(fmt.Sprintf("\tif hasVersioningMarker(\"%s\") {\n", resource))
		registrations.WriteString(fmt.Sprintf("\t\tgen.SetResourceTag(\"%s\", \"versioning\", \"enabled\")\n", resource))
		registrations.WriteString("\t}\n")
	}

	return fmt.Sprintf(`// Code generated by fabrica codegen init. DO NOT EDIT.
package resources

import (
	"fmt"
		"os"
		"path/filepath"
		"strings"

	"github.com/openchami/fabrica/pkg/codegen"
%s)

// RegisterAllResources registers all discovered resources with the generator.
// This file is auto-generated. Re-run 'fabrica codegen init' after adding resources.
func RegisterAllResources(gen *codegen.Generator) error {
%s
	return nil
}

	// hasVersioningMarker inspects the resource source file for the versioning marker comment.
	func hasVersioningMarker(resourceName string) bool {
		// Derive path: pkg/resources/<lower(resourceName)>/<lower(resourceName)>.go
		pkg := strings.ToLower(resourceName)
		path := filepath.Join("pkg", "resources", pkg, pkg+".go")
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		content := string(data)
		return strings.Contains(content, "+fabrica:resource-versioning=enabled")
	}
`, imports.String(), registrations.String())
}

// generateVersionedRegistrationCode creates registration code for versioned (apis/) mode
func generateVersionedRegistrationCode(modulePath string, config *FabricaConfig, resources []string) string {
	var imports strings.Builder
	var registrations strings.Builder

	hubVersion := config.Features.Versioning.StorageVersion
	group := config.Features.Versioning.Group

	for _, resource := range resources {
		pkg := hubVersion // Package name is the version (e.g., v1)
		// Import path: module/apis/group/version
		importPath := fmt.Sprintf("%s/apis/%s/%s", modulePath, group, hubVersion)

		imports.WriteString(fmt.Sprintf("\t%s \"%s\"\n", pkg, importPath))
		registrations.WriteString(fmt.Sprintf("\tif err := gen.RegisterResource(&%s.%s{}); err != nil {\n", pkg, resource))
		registrations.WriteString(fmt.Sprintf("\t\treturn fmt.Errorf(\"failed to register %s: %%w\", err)\n", resource))
		registrations.WriteString("\t}\n")
	}

	return fmt.Sprintf(`// Code generated by fabrica. DO NOT EDIT.
package resources

import (
	"fmt"

	"github.com/openchami/fabrica/pkg/codegen"
%s)

// RegisterAllResources registers all discovered resources with the generator.
// This file is auto-generated. Re-run 'fabrica generate' after adding resources.
func RegisterAllResources(gen *codegen.Generator) error {
%s
	return nil
}
`, imports.String(), registrations.String())
}

// generateEntCode runs 'go generate ./internal/storage' to generate Ent client code
// This is automatically called by 'fabrica generate' when Ent storage is detected
func generateEntCode(debug bool) error {
	// Check prerequisites
	if _, err := os.Stat("internal/storage/ent/schema"); os.IsNotExist(err) {
		return fmt.Errorf("ent schema directory not found")
	}

	if _, err := os.Stat("internal/storage/generate.go"); os.IsNotExist(err) {
		return fmt.Errorf("generate.go not found in internal/storage")
	}

	// Run go generate
	entCmd := exec.Command("go", "generate", "./internal/storage")
	if debug {
		entCmd.Stdout = os.Stdout
		entCmd.Stderr = os.Stderr
	}

	if err := entCmd.Run(); err != nil {
		return err
	}

	if !debug {
		fmt.Println("✅ Ent client code generated")
	}

	return nil
}

// Version comparison and checking functions

// parseVersion extracts version from a string like "v1.2.3" or "1.2.3"
// Returns major, minor, patch as integers
func parseVersion(v string) (major, minor, patch int, err error) {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	// Handle "dev" or empty versions
	if v == "" || v == "dev" || v == "none" {
		return 0, 0, 0, nil
	}

	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, 0, fmt.Errorf("invalid version format: %s", v)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	if len(parts) >= 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}

	return major, minor, patch, nil
}

// detectGeneratedVersion looks for existing generated files and extracts the version
// Returns the version string found, or empty string if none found
func detectGeneratedVersion() string {
	// Check common generated files for version info
	filesToCheck := []string{
		"cmd/server/routes_generated.go",
		"cmd/server/models_generated.go",
		"internal/storage/storage_generated.go",
		"pkg/client/client_generated.go",
	}

	for _, file := range filesToCheck {
		if data, err := os.ReadFile(file); err == nil {
			// Look for "Code generated by Fabrica VERSION" pattern
			content := string(data)
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				if strings.Contains(line, "Code generated by Fabrica") ||
					strings.Contains(line, "Generated by Fabrica") {
					// Extract version - format: "Fabrica v1.2.3" or "Fabrica dev"
					fields := strings.Fields(line)
					for i, field := range fields {
						if field == "Fabrica" && i+1 < len(fields) {
							version := strings.TrimSuffix(fields[i+1], ".")
							return version
						}
					}
				}
			}
		}
	}

	return ""
}

// checkVersionCompatibility checks if regeneration should proceed
// Returns true if safe to proceed, false if --force is required
func checkVersionCompatibility(currentVer, generatedVer string, force bool) (bool, error) {
	// If no generated version found, warn but require --force
	if generatedVer == "" {
		if force {
			return true, nil
		}
		fmt.Println()
		fmt.Println("⚠️  WARNING: Could not detect version of existing generated code")
		fmt.Println("   This may be an older project without version tracking.")
		fmt.Println("   Generated code will be updated to include version information.")
		fmt.Println()
		fmt.Println("   Use --force to proceed with regeneration")
		fmt.Println()
		return false, nil
	}

	// If force flag is set, always allow
	if force {
		return true, nil
	}

	// Special case: "dev" version is treated as latest/unreleased
	// Always allow regeneration with dev version
	if currentVer == "dev" || currentVer == "none" {
		return true, nil
	}

	// Parse versions
	currMajor, currMinor, _, err := parseVersion(currentVer)
	if err != nil {
		// Can't parse current version, allow with warning
		fmt.Printf("⚠️  Warning: Could not parse current version: %s\n", currentVer)
		return true, nil
	}

	genMajor, genMinor, _, err := parseVersion(generatedVer)
	if err != nil {
		// Can't parse generated version, allow with warning
		fmt.Printf("⚠️  Warning: Could not parse generated version: %s\n", generatedVer)
		return true, nil
	}

	// Rule 1: Generated version is higher or equal to current minor version
	if genMajor > currMajor || (genMajor == currMajor && genMinor >= currMinor) {
		fmt.Println()
		fmt.Printf("⚠️  WARNING: Generated code is from Fabrica %s\n", generatedVer)
		fmt.Printf("   Current Fabrica version is %s\n", currentVer)
		fmt.Println()
		fmt.Println("   You are trying to regenerate code with an OLDER or SAME version of Fabrica.")
		fmt.Println("   This may cause regressions or unexpected behavior.")
		fmt.Println()
		fmt.Println("   Use --force to proceed with regeneration")
		fmt.Println()
		return false, nil
	}

	// Rule 2: Generated version is more than one major version behind
	if currMajor > genMajor+1 {
		fmt.Println()
		fmt.Printf("⚠️  WARNING: Generated code is from Fabrica %s\n", generatedVer)
		fmt.Printf("   Current Fabrica version is %s\n", currentVer)
		fmt.Println()
		fmt.Printf("   Generated code is MORE THAN ONE MAJOR VERSION behind (%d major versions)\n", currMajor-genMajor)
		fmt.Println("   This may require manual migration steps.")
		fmt.Println()
		fmt.Println("   Use --force to proceed with regeneration")
		fmt.Println("   Review the CHANGELOG and migration guides first:")
		fmt.Printf("   https://github.com/openchami/fabrica/blob/main/CHANGELOG.md\n")
		fmt.Println()
		return false, nil
	}

	// Safe to proceed
	return true, nil
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type initOptions struct {
	mode         string // simple, standard, expert
	interactive  bool
	modulePath   string
	withExamples bool
	withDocs     bool
	withAuth     bool   // Enable Casbin authorization
	storageType  string // file, ent
	dbDriver     string // postgres, mysql, sqlite
}

func newInitCommand() *cobra.Command {
	opts := &initOptions{}

	cmd := &cobra.Command{
		Use:   "init [project-name]",
		Short: "Initialize a new Fabrica project",
		Long: `Initialize a new Fabrica project with tiered complexity levels.

Modes:
  simple   - Basic REST API without Kubernetes concepts (quick start)
  standard - Full resource model with labels, annotations (recommended)
  expert   - Minimal scaffolding, maximum flexibility

The interactive flag launches a guided wizard to help you choose.

You can initialize in an existing directory by using '.' as the project name,
or by providing the name of an existing directory. This is useful when using
'gh repo create --template' or similar workflows.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			projectName := "myproject"
			if len(args) > 0 {
				projectName = args[0]
			}

			// If a non-default database driver is specified, automatically use ent storage
			// (sqlite is the default, so we check if postgres or mysql was explicitly chosen)
			if opts.dbDriver == "postgres" || opts.dbDriver == "mysql" {
				opts.storageType = "ent"
			}

			if opts.interactive {
				return runInteractiveInit(projectName, opts)
			}

			return runInit(projectName, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.mode, "mode", "m", "standard", "Project mode: simple, standard, or expert")
	cmd.Flags().BoolVarP(&opts.interactive, "interactive", "i", false, "Interactive wizard mode")
	cmd.Flags().StringVar(&opts.modulePath, "module", "", "Go module path (e.g., github.com/user/project)")
	cmd.Flags().BoolVar(&opts.withExamples, "examples", true, "Include example code")
	cmd.Flags().BoolVar(&opts.withDocs, "docs", true, "Generate documentation")
	cmd.Flags().BoolVar(&opts.withAuth, "auth", false, "Enable Casbin authorization policies")
	cmd.Flags().StringVar(&opts.storageType, "storage", "file", "Storage backend: file or ent")
	cmd.Flags().StringVar(&opts.dbDriver, "db", "sqlite", "Database driver for Ent: postgres, mysql, or sqlite")

	return cmd
}

func runInteractiveInit(projectName string, opts *initOptions) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("🏗️  Welcome to Fabrica!")
	fmt.Println()
	fmt.Println("Let's set up your project. I'll ask a few questions to customize it for you.")
	fmt.Println()

	// Project name
	if projectName == "myproject" {
		fmt.Print("Project name: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			projectName = input
		}
	}

	// Module path
	if opts.modulePath == "" {
		fmt.Printf("Go module path (e.g., github.com/user/%s): ", projectName)
		input, _ := reader.ReadString('\n')
		opts.modulePath = strings.TrimSpace(input)
		if opts.modulePath == "" {
			opts.modulePath = fmt.Sprintf("github.com/user/%s", projectName)
		}
	}

	// Experience level
	fmt.Println()
	fmt.Println("What's your experience level with Fabrica and Kubernetes?")
	fmt.Println("  1) Beginner - Just want a simple REST API")
	fmt.Println("  2) Intermediate - Familiar with REST, want resource management")
	fmt.Println("  3) Advanced - Know Kubernetes, want full power")
	fmt.Print("Choice [2]: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		opts.mode = "simple"
	case "3":
		opts.mode = "expert"
	default:
		opts.mode = "standard"
	}

	// Storage backend
	fmt.Println()
	fmt.Println("Which storage backend do you want?")
	fmt.Println("  1) File - Simple file-based storage (default)")
	fmt.Println("  2) Ent - Database storage with PostgreSQL/MySQL/SQLite")
	fmt.Print("Choice [1]: ")

	choice, _ = reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "2" {
		opts.storageType = "ent"

		fmt.Println()
		fmt.Println("Which database driver?")
		fmt.Println("  1) SQLite - Embedded database (great for development)")
		fmt.Println("  2) PostgreSQL - Production-ready database")
		fmt.Println("  3) MySQL - Alternative production database")
		fmt.Print("Choice [1]: ")

		dbChoice, _ := reader.ReadString('\n')
		dbChoice = strings.TrimSpace(dbChoice)

		switch dbChoice {
		case "2":
			opts.dbDriver = "postgres"
		case "3":
			opts.dbDriver = "mysql"
		default:
			opts.dbDriver = "sqlite"
		}
	} else {
		opts.storageType = "file"
	}

	// Features
	fmt.Println()
	fmt.Print("Include example code? [Y/n]: ")
	input, _ := reader.ReadString('\n')
	opts.withExamples = !strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "n")

	fmt.Print("Generate documentation? [Y/n]: ")
	input, _ = reader.ReadString('\n')
	opts.withDocs = !strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "n")

	// Summary
	fmt.Println()
	fmt.Println("📋 Summary:")
	fmt.Printf("  Project: %s\n", projectName)
	fmt.Printf("  Module: %s\n", opts.modulePath)
	fmt.Printf("  Mode: %s\n", opts.mode)
	fmt.Printf("  Storage: %s", opts.storageType)
	if opts.storageType == "ent" {
		fmt.Printf(" (%s)", opts.dbDriver)
	}
	fmt.Println()
	fmt.Printf("  Examples: %v\n", opts.withExamples)
	fmt.Printf("  Docs: %v\n", opts.withDocs)
	fmt.Println()
	fmt.Print("Proceed? [Y/n]: ")

	input, _ = reader.ReadString('\n')
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "n") {
		fmt.Println("Cancelled.")
		return nil
	}

	return runInit(projectName, opts)
}

func runInit(projectName string, opts *initOptions) error {
	// Determine if we're initializing in current directory
	inCurrentDir := projectName == "."
	var projectBaseName string
	var targetDir string

	if inCurrentDir {
		// Initialize in current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		projectBaseName = filepath.Base(cwd)
		targetDir = "."
		fmt.Printf("🚀 Initializing Fabrica project in current directory (%s) in %s mode...\n", projectBaseName, opts.mode)

		// Check if current directory already has Fabrica files
		if err := checkExistingProject("."); err != nil {
			return err
		}

		// Check if directory has important files we should preserve
		if err := checkSafeToInitialize("."); err != nil {
			return err
		}

		fmt.Println("📁 Initializing in existing directory...")
	} else {
		// Creating or initializing in a named directory
		projectBaseName = filepath.Base(projectName)
		targetDir = projectName

		if stat, err := os.Stat(projectName); err == nil && stat.IsDir() {
			// Directory exists
			fmt.Printf("🚀 Initializing Fabrica project in existing directory %s in %s mode...\n", projectName, opts.mode)

			// Check if directory already has Fabrica files
			if err := checkExistingProject(projectName); err != nil {
				return err
			}

			// Check if directory has important files we should preserve
			if err := checkSafeToInitialize(projectName); err != nil {
				return err
			}

			fmt.Println("📁 Initializing in existing directory...")
		} else {
			// Create new directory
			fmt.Printf("🚀 Creating %s project in %s mode...\n", projectName, opts.mode)
			if err := os.MkdirAll(projectName, 0755); err != nil {
				return fmt.Errorf("failed to create project directory: %w", err)
			}
		}
	}

	// Create directory structure
	dirs := []string{
		"cmd/server",
		"cmd/client",
		"pkg/resources",
		"internal/storage",
		"api/v1",
	}

	if opts.withExamples {
		dirs = append(dirs, "examples")
	}

	if opts.withDocs {
		dirs = append(dirs, "docs")
	}

	for _, dir := range dirs {
		path := filepath.Join(targetDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create go.mod (only if it doesn't exist)
	if err := createGoMod(targetDir, projectBaseName, opts.modulePath, opts); err != nil {
		return err
	}

	// Create files based on mode
	switch opts.mode {
	case "simple":
		if err := createSimpleModeFiles(targetDir, opts); err != nil {
			return err
		}
	case "expert":
		if err := createExpertModeFiles(targetDir, opts); err != nil {
			return err
		}
	default: // standard
		if err := createStandardModeFiles(targetDir, opts); err != nil {
			return err
		}
	}

	// Create README
	if err := createREADME(targetDir, projectBaseName, opts); err != nil {
		return err
	}

	// Create Makefile
	if err := createMakefile(targetDir, opts); err != nil {
		return err
	}

	// Create policy files if auth is enabled
	if opts.withAuth {
		if err := createPolicyFiles(targetDir, opts); err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Println("✅ Project created successfully!")
	fmt.Println()
	fmt.Println("Next steps:")

	// Only show cd command if not in current directory
	if !inCurrentDir {
		fmt.Printf("  cd %s\n", projectName)
	}

	fmt.Println("  go mod tidy")

	if opts.mode == "simple" {
		fmt.Println("  fabrica add resource Product    # Add your first resource")
		fmt.Println("  fabrica generate                # Generate code")
	} else {
		fmt.Println("  fabrica add resource Device     # Add your first resource")
		fmt.Println("  fabrica generate                # Generate handlers and storage")
	}

	fmt.Println("  go run cmd/server/main.go       # Start the server")
	fmt.Println()

	if opts.withDocs {
		if inCurrentDir {
			fmt.Println("📚 Documentation available in docs/")
		} else {
			fmt.Printf("📚 Documentation available in %s/docs/\n", projectName)
		}
	}

	if opts.withExamples {
		if inCurrentDir {
			fmt.Println("💡 Examples available in examples/")
		} else {
			fmt.Printf("💡 Examples available in %s/examples/\n", projectName)
		}
	}

	return nil
}

func createGoMod(targetDir, projectName, modulePath string, opts *initOptions) error {
	// Check if go.mod already exists
	goModPath := filepath.Join(targetDir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		fmt.Println("ℹ️  go.mod already exists, skipping...")
		return nil
	}

	if modulePath == "" {
		modulePath = fmt.Sprintf("github.com/user/%s", projectName)
	}

	content := fmt.Sprintf(`module %s

go 1.23

require (
	github.com/alexlovelltroy/fabrica %s
	github.com/getkin/kin-openapi v0.128.0`, modulePath, getFabricaVersion())

	// Add Ent dependencies if using Ent storage
	if opts.storageType == "ent" {
		content += `
	entgo.io/ent v0.14.1`

		switch opts.dbDriver {
		case "postgres":
			content += `
	github.com/lib/pq v1.10.9`
		case "mysql":
			content += `
	github.com/go-sql-driver/mysql v1.8.1`
		case "sqlite":
			content += `
	github.com/mattn/go-sqlite3 v1.14.24`
		}
	}

	// Add Casbin dependency if authorization is enabled
	if opts.withAuth {
		content += `
	github.com/casbin/casbin/v2 v2.102.0`
	}

	content += `
)
`

	return os.WriteFile(goModPath, []byte(content), 0644)
}

func createREADME(targetDir, projectName string, opts *initOptions) error {
	// Check if README already exists
	readmePath := filepath.Join(targetDir, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		fmt.Println("ℹ️  README.md already exists, skipping...")
		return nil
	}
	modeDesc := map[string]string{
		"simple":   "Simple REST API",
		"standard": "Resource-based REST API with Fabrica",
		"expert":   "Advanced Fabrica Project",
	}

	content := fmt.Sprintf(`# %s

%s created with Fabrica.

## Quick Start

`, projectName, modeDesc[opts.mode])

	if opts.mode == "simple" {
		content += `### Add a Resource

` + "```bash" + `
fabrica add resource Product
` + "```" + `

### Generate Code

` + "```bash" + `
fabrica generate
` + "```" + `

### Run

` + "```bash" + `
go run cmd/server/main.go
` + "```" + `

Your API will be available at http://localhost:8080
`
	} else {
		content += `### Generate Handlers

` + "```bash" + `
fabrica generate
` + "```" + `

### Run the Server

` + "```bash" + `
go run cmd/server/main.go
` + "```" + `

## Documentation

See [docs/](docs/) for detailed documentation.
`
	}

	return os.WriteFile(readmePath, []byte(content), 0644)
}

func createMakefile(projectName string, _ *initOptions) error {
	content := `.PHONY: build run test generate clean dev codegen-init

build:
	go build -o bin/server cmd/server/main.go

run: build
	./bin/server

test:
	go test ./...

# Initialize code generation (run after adding resources)
codegen-init:
	fabrica codegen init

# Generate handlers, storage, and OpenAPI specs
generate:
	fabrica generate --handlers --storage --openapi

# Development workflow: regenerate and build
dev: clean codegen-init generate build
	@echo "✅ Development build complete"

clean:
	rm -rf bin/
	rm -f cmd/server/*_generated.go
	rm -f internal/storage/storage_generated.go
	rm -f pkg/client/*_generated.go
	rm -f pkg/resources/register_generated.go
`

	path := filepath.Join(projectName, "Makefile")
	return os.WriteFile(path, []byte(content), 0644)
}

func createSimpleModeFiles(projectName string, opts *initOptions) error {
	// Create a simplified main.go
	mainContent := `package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Add your routes here
	// Example: r.Get("/resources", listResources)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
`

	path := filepath.Join(projectName, "cmd/server/main.go")
	if err := os.WriteFile(path, []byte(mainContent), 0644); err != nil {
		return err
	}

	// Create simple docs
	if opts.withDocs {
		docsContent := `# Simple Mode Documentation

In simple mode, Fabrica acts as a lightweight code generator.

## Adding Resources

` + "```bash" + `
fabrica add resource Product
` + "```" + `

This creates a resource definition and generates CRUD handlers.

## Next Steps

- Add validation with struct tags
- Add business logic in handlers
- Explore standard mode for advanced features
`
		docsPath := filepath.Join(projectName, "docs/getting-started.md")
		if err := os.WriteFile(docsPath, []byte(docsContent), 0644); err != nil {
			return err
		}
	}

	return nil
}

func createStandardModeFiles(projectName string, opts *initOptions) error {
	// Create standard main.go with full Fabrica features
	mainContent := `package main

import (
	"log"
	"net/http"

	"github.com/alexlovelltroy/fabrica/pkg/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Initialize storage backend
	backend, err := storage.NewFileBackend("./data")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Setup router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Register generated routes here
	// RegisterRoutes(r, backend)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
`

	path := filepath.Join(projectName, "cmd/server/main.go")
	if err := os.WriteFile(path, []byte(mainContent), 0644); err != nil {
		return err
	}

	// Create standard mode documentation
	if opts.withDocs {
		docsContent := `# Standard Mode Documentation

Standard mode provides the full Fabrica resource model with:

- Labels and annotations
- Conditions and status
- Multi-version support
- Event system
- Storage abstraction

## Resource Model

Resources follow the Kubernetes pattern:

` + "```go" + `
type MyResource struct {
    resource.Resource
    Spec   MyResourceSpec   ` + "`json:\"spec\"`" + `
    Status MyResourceStatus ` + "`json:\"status,omitempty\"`" + `
}
` + "```" + `

## Next Steps

- Read the [Resource Guide](resource-guide.md)
- Explore [Validation](validation.md)
- Learn about [Events](events.md)
`
		docsPath := filepath.Join(projectName, "docs/getting-started.md")
		if err := os.WriteFile(docsPath, []byte(docsContent), 0644); err != nil {
			return err
		}
	}

	return nil
}

func createExpertModeFiles(projectName string, _ *initOptions) error {
	// Minimal scaffolding for expert mode
	mainContent := `package main

import (
	"log"
	"net/http"
)

func main() {
	// Build your application here

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
`

	path := filepath.Join(projectName, "cmd/server/main.go")
	return os.WriteFile(path, []byte(mainContent), 0644)
}

// checkExistingProject checks if the directory already contains a Fabrica project
func checkExistingProject(dir string) error {
	// Check for key Fabrica files that indicate this is already initialized
	fabricaFiles := []string{
		"cmd/server/main.go",
		"pkg/resources",
	}

	for _, file := range fabricaFiles {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("directory appears to already contain a Fabrica project (found %s)\nUse a different directory or remove existing files first", file)
		}
	}

	return nil
}

// checkSafeToInitialize verifies that we won't overwrite important files
func checkSafeToInitialize(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// List of files/directories that are safe to ignore
	safeFiles := map[string]bool{
		".git":           true,
		".gitignore":     true,
		".github":        true,
		"LICENSE":        true,
		"LICENSES":       true,
		"README.md":      true,
		".gitattributes": true,
		".editorconfig":  true,
		".vscode":        true,
		".idea":          true,
	}

	// Check for potentially problematic files
	hasUnsafeFiles := false
	unsafeFiles := []string{}

	for _, entry := range entries {
		name := entry.Name()

		// Skip safe files
		if safeFiles[name] {
			continue
		}

		// Skip hidden files and directories (except the ones we explicitly check)
		if strings.HasPrefix(name, ".") {
			continue
		}

		// If we find any other files, warn the user
		hasUnsafeFiles = true
		unsafeFiles = append(unsafeFiles, name)
	}

	if hasUnsafeFiles {
		fmt.Println("⚠️  Warning: Directory contains existing files:")
		for _, f := range unsafeFiles {
			fmt.Printf("    - %s\n", f)
		}
		fmt.Println()
		fmt.Print("Continue and potentially overwrite files? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			return fmt.Errorf("initialization cancelled by user")
		}
		fmt.Println()
	}

	return nil
}

// createPolicyFiles creates Casbin policy files for authorization
func createPolicyFiles(projectName string, _ *initOptions) error {
	// Create policies directory
	policyDir := filepath.Join(projectName, "policies")
	if err := os.MkdirAll(policyDir, 0755); err != nil {
		return fmt.Errorf("failed to create policies directory: %w", err)
	}

	// Create model.conf - Casbin RBAC model
	modelContent := `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act || g(r.sub, p.sub) && p.obj == "*" && r.act == p.act || g(r.sub, p.sub) && r.obj == p.obj && p.act == "*"
`

	modelPath := filepath.Join(policyDir, "model.conf")
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		return fmt.Errorf("failed to create model.conf: %w", err)
	}

	// Create policy.csv - Default policies
	policyContent := `# Casbin Policy File
# Format: p, subject, object, action
# Format: g, user, role

# Admin role - full access to all resources
p, admin, *, *

# User role - read-only access
p, user, *, list
p, user, *, get

# Example: grant admin role to a specific user
# g, user:alice@example.com, admin

# Example: grant user role to a specific user
# g, user:bob@example.com, user
`

	policyPath := filepath.Join(policyDir, "policy.csv")
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		return fmt.Errorf("failed to create policy.csv: %w", err)
	}

	fmt.Println("  ├─ Created Casbin policy files")
	return nil
}

// getFabricaVersion returns the version string to use in go.mod
func getFabricaVersion() string {
	// version is set via ldflags at build time
	if version != "" && version != "dev" {
		return version
	}

	// Fallback: try to get from current module
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/alexlovelltroy/fabrica")
	if output, err := cmd.Output(); err == nil {
		v := strings.TrimSpace(string(output))
		if v != "" && v != "(devel)" {
			return v
		}
	}

	// Last resort: use latest stable
	return "v0.2.3"
}

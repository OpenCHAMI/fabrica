// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"os"
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

The interactive flag launches a guided wizard to help you choose.`,
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
	fmt.Printf("🚀 Creating %s project in %s mode...\n", projectName, opts.mode)

	// Create project directory
	if err := os.MkdirAll(projectName, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
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
		path := filepath.Join(projectName, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create go.mod
	if err := createGoMod(projectName, opts.modulePath, opts); err != nil {
		return err
	}

	// Create files based on mode
	switch opts.mode {
	case "simple":
		if err := createSimpleModeFiles(projectName, opts); err != nil {
			return err
		}
	case "expert":
		if err := createExpertModeFiles(projectName, opts); err != nil {
			return err
		}
	default: // standard
		if err := createStandardModeFiles(projectName, opts); err != nil {
			return err
		}
	}

	// Create README
	if err := createREADME(projectName, opts); err != nil {
		return err
	}

	// Create Makefile
	if err := createMakefile(projectName, opts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✅ Project created successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectName)
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
		fmt.Printf("📚 Documentation available in %s/docs/\n", projectName)
	}

	if opts.withExamples {
		fmt.Printf("💡 Examples available in %s/examples/\n", projectName)
	}

	return nil
}

func createGoMod(projectName, modulePath string, opts *initOptions) error {
	if modulePath == "" {
		modulePath = fmt.Sprintf("github.com/user/%s", projectName)
	}

	content := fmt.Sprintf(`module %s

go 1.23

require (
	github.com/alexlovelltroy/fabrica latest`, modulePath)

	// Add Ent dependencies if using Ent storage
	if opts.storageType == "ent" {
		content += `
	entgo.io/ent latest`

		switch opts.dbDriver {
		case "postgres":
			content += `
	github.com/lib/pq latest`
		case "mysql":
			content += `
	github.com/go-sql-driver/mysql latest`
		case "sqlite":
			content += `
	github.com/mattn/go-sqlite3 latest`
		}
	}

	content += `
)
`

	path := filepath.Join(projectName, "go.mod")
	return os.WriteFile(path, []byte(content), 0644)
}

func createREADME(projectName string, opts *initOptions) error {
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

	path := filepath.Join(projectName, "README.md")
	return os.WriteFile(path, []byte(content), 0644)
}

func createMakefile(projectName string, _ *initOptions) error {
	content := `.PHONY: build run test generate clean

build:
	go build -o bin/server cmd/server/main.go

run:
	go run cmd/server/main.go

test:
	go test ./...

generate:
	fabrica generate

clean:
	rm -rf bin/
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

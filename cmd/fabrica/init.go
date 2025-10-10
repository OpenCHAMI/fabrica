// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

//go:embed main_cobra.go.tmpl
var mainCobraTemplate string

type initOptions struct {
	interactive bool
	modulePath  string
	description string

	// Feature flags instead of modes
	withAuth      bool // Enable authentication
	withStorage   bool // Enable storage backend
	withHSM       bool // Enable HSM support
	withLegacyAPI bool // Enable legacy API compatibility
	withMetrics   bool // Enable metrics/monitoring
	withVersion   bool // Enable version command

	// New feature flags for core features
	validationMode  string // strict, warn, disabled
	withEvents      bool   // Enable CloudEvents support
	eventBusType    string // memory, nats, kafka
	versionStrategy string // header, url, both

	// Storage options
	storageType string // file, ent
	dbDriver    string // postgres, mysql, sqlite
}

// Template data structure
type templateData struct {
	ProjectName   string
	ModulePath    string
	Description   string
	WithAuth      bool
	WithStorage   bool
	WithHSM       bool
	WithLegacyAPI bool
	WithMetrics   bool
	WithVersion   bool
	StorageType   string
	DBDriver      string
}

func newInitCommand() *cobra.Command {
	opts := &initOptions{
		withStorage:     true,     // Default to enabling storage
		withVersion:     true,     // Default to enabling version command
		storageType:     "file",   // Default to file storage
		dbDriver:        "sqlite", // Default database
		validationMode:  "strict", // Default validation mode
		eventBusType:    "memory", // Default event bus
		versionStrategy: "header", // Default version strategy
	}

	cmd := &cobra.Command{
		Use:   "init [project-name]",
		Short: "Initialize a new Fabrica project",
		Long: `Initialize a new Fabrica project with configurable features.

Instead of complex modes, use feature flags to customize your project:
  --auth          Enable authentication with TokenSmith
  --storage       Enable persistent storage (file or database)
  --hsm           Enable HSM support for cryptographic operations
  --legacy-api    Enable legacy API compatibility
  --metrics       Enable Prometheus metrics

The interactive flag launches a guided wizard to help you choose.

You can initialize in an existing directory by using '.' as the project name,
or by providing the name of an existing directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			projectName := "myproject"
			if len(args) > 0 {
				projectName = args[0]
			}

			// If a non-default database driver is specified, automatically use ent storage
			if opts.dbDriver == "postgres" || opts.dbDriver == "mysql" {
				opts.storageType = "ent"
			}

			if opts.interactive {
				return runInteractiveInit(projectName, opts)
			}

			return runInit(projectName, opts)
		},
	}

	// Feature flags instead of complex modes
	cmd.Flags().BoolVarP(&opts.interactive, "interactive", "i", false, "Interactive wizard mode")
	cmd.Flags().StringVar(&opts.modulePath, "module", "", "Go module path (e.g., github.com/user/project)")
	cmd.Flags().StringVar(&opts.description, "description", "", "Project description")

	// Feature flags
	cmd.Flags().BoolVar(&opts.withAuth, "auth", false, "Enable authentication with TokenSmith")
	cmd.Flags().BoolVar(&opts.withStorage, "storage", true, "Enable persistent storage")
	cmd.Flags().BoolVar(&opts.withHSM, "hsm", false, "Enable HSM support for cryptographic operations")
	cmd.Flags().BoolVar(&opts.withLegacyAPI, "legacy-api", false, "Enable legacy API compatibility")
	cmd.Flags().BoolVar(&opts.withMetrics, "metrics", false, "Enable Prometheus metrics")
	cmd.Flags().BoolVar(&opts.withVersion, "version", true, "Enable version command")

	// Core feature configuration
	cmd.Flags().StringVar(&opts.validationMode, "validation-mode", "strict", "Validation mode: strict, warn, or disabled")
	cmd.Flags().BoolVar(&opts.withEvents, "events", false, "Enable CloudEvents support")
	cmd.Flags().StringVar(&opts.eventBusType, "events-bus", "memory", "Event bus type: memory, nats, or kafka")
	cmd.Flags().StringVar(&opts.versionStrategy, "version-strategy", "header", "API versioning strategy: header, url, or both")

	// Storage options
	cmd.Flags().StringVar(&opts.storageType, "storage-type", "file", "Storage backend: file or ent")
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

	// Description
	fmt.Printf("Project description (optional): ")
	input, _ := reader.ReadString('\n')
	opts.description = strings.TrimSpace(input)

	// Features
	fmt.Println()
	fmt.Println("🚀 Features to enable:")

	// Authentication
	fmt.Print("Enable authentication with TokenSmith? [y/N]: ")
	input, _ = reader.ReadString('\n')
	opts.withAuth = strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "y")

	// Storage
	fmt.Print("Enable persistent storage? [Y/n]: ")
	input, _ = reader.ReadString('\n')
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "n") {
		opts.withStorage = false
	} else {
		opts.withStorage = true

		// Storage type
		fmt.Println("Storage backend:")
		fmt.Println("  1) File-based storage (simple)")
		fmt.Println("  2) Database with Ent (postgres/mysql/sqlite)")
		fmt.Print("Choose [1]: ")
		input, _ = reader.ReadString('\n')
		switch strings.TrimSpace(input) {
		case "2":
			opts.storageType = "ent"

			// Database driver
			fmt.Println("Database driver:")
			fmt.Println("  1) SQLite (file-based)")
			fmt.Println("  2) PostgreSQL")
			fmt.Println("  3) MySQL")
			fmt.Print("Choose [1]: ")
			input, _ = reader.ReadString('\n')
			switch strings.TrimSpace(input) {
			case "2":
				opts.dbDriver = "postgres"
			case "3":
				opts.dbDriver = "mysql"
			default:
				opts.dbDriver = "sqlite"
			}
		default:
			opts.storageType = "file"
		}
	}

	// HSM support
	fmt.Print("Enable HSM support for cryptographic operations? [y/N]: ")
	input, _ = reader.ReadString('\n')
	opts.withHSM = strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "y")

	// Legacy API
	fmt.Print("Enable legacy API compatibility? [y/N]: ")
	input, _ = reader.ReadString('\n')
	opts.withLegacyAPI = strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "y")

	// Metrics
	fmt.Print("Enable Prometheus metrics? [y/N]: ")
	input, _ = reader.ReadString('\n')
	opts.withMetrics = strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "y")

	// Summary
	fmt.Println()
	fmt.Println("📋 Summary:")
	fmt.Printf("  Project: %s\n", projectName)
	fmt.Printf("  Module: %s\n", opts.modulePath)
	if opts.description != "" {
		fmt.Printf("  Description: %s\n", opts.description)
	}
	fmt.Printf("  Features:\n")
	fmt.Printf("    Authentication: %s\n", map[bool]string{true: "enabled", false: "disabled"}[opts.withAuth])
	if opts.withStorage {
		fmt.Printf("    Storage: %s", opts.storageType)
		if opts.storageType == "ent" {
			fmt.Printf(" (%s)", opts.dbDriver)
		}
		fmt.Println()
	} else {
		fmt.Printf("    Storage: disabled\n")
	}
	fmt.Printf("    HSM Support: %s\n", map[bool]string{true: "enabled", false: "disabled"}[opts.withHSM])
	fmt.Printf("    Legacy API: %s\n", map[bool]string{true: "enabled", false: "disabled"}[opts.withLegacyAPI])
	fmt.Printf("    Metrics: %s\n", map[bool]string{true: "enabled", false: "disabled"}[opts.withMetrics])

	fmt.Print("\nProceed? [Y/n]: ")
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
		fmt.Printf("🚀 Initializing Fabrica project in current directory (%s)...\n", projectBaseName)

		// Check if current directory already has Fabrica files
		if err := checkExistingProject("."); err != nil {
			return err
		}
	} else {
		// Check if directory already exists
		if _, err := os.Stat(projectName); err == nil {
			// Directory exists, initialize within it
			if err := checkExistingProject(projectName); err != nil {
				return err
			}
			fmt.Printf("🚀 Initializing Fabrica project in existing directory %s...\n", projectName)
		} else {
			// Create new directory
			fmt.Printf("🚀 Creating %s project...\n", projectName)
		}
		projectBaseName = projectName
		targetDir = projectName
	}

	// Set default module path if not provided
	if opts.modulePath == "" {
		opts.modulePath = fmt.Sprintf("github.com/user/%s", projectBaseName)
	}

	// Create project structure
	if err := createProjectStructure(targetDir, projectBaseName, opts); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	// Success message
	fmt.Println()
	fmt.Println("✅ Project initialized successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Define your resources in pkg/resources/")
	fmt.Println("  2. Run 'fabrica generate' to generate code")
	fmt.Println("  3. Start development with 'go run cmd/server/main.go'")
	fmt.Println()

	return nil
}

func createProjectStructure(targetDir, projectName string, opts *initOptions) error {
	// Normalize database driver name (sqlite -> sqlite3 for Go driver compatibility)
	dbDriver := opts.dbDriver
	if dbDriver == "sqlite" {
		dbDriver = "sqlite3"
	}

	// Template data
	data := templateData{
		ProjectName:   projectName,
		ModulePath:    opts.modulePath,
		Description:   opts.description,
		WithAuth:      opts.withAuth,
		WithStorage:   opts.withStorage,
		WithHSM:       opts.withHSM,
		WithLegacyAPI: opts.withLegacyAPI,
		WithMetrics:   opts.withMetrics,
		WithVersion:   opts.withVersion,
		StorageType:   opts.storageType,
		DBDriver:      dbDriver,
	}

	// Create directories
	dirs := []string{
		"cmd/server",
		"pkg/resources",
		"internal/storage",
	}

	for _, dir := range dirs {
		path := filepath.Join(targetDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	// Generate main.go from template
	if err := generateFromTemplate("main_cobra.go.tmpl", filepath.Join(targetDir, "cmd/server/main.go"), data); err != nil {
		return err
	}

	// Create go.mod
	if err := createGoMod(targetDir, opts.modulePath); err != nil {
		return err
	}

	// Create basic files
	if err := createBasicFiles(targetDir, data); err != nil {
		return err
	}

	// Create Fabrica configuration file
	if err := createFabricaConfig(targetDir, opts); err != nil {
		return err
	}

	return nil
}

func generateFromTemplate(templateName, outputPath string, data templateData) error {
	var tmplContent string

	// Use the embedded template
	if templateName == "main_cobra.go.tmpl" {
		tmplContent = mainCobraTemplate
	} else {
		return fmt.Errorf("template %s not found", templateName)
	}

	// Template functions
	funcMap := template.FuncMap{
		"toLower": strings.ToLower,
		"toUpper": strings.ToUpper,
	}

	tmpl, err := template.New(templateName).Funcs(funcMap).Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", outputPath, err)
	}
	defer file.Close() //nolint:errcheck

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func createGoMod(targetDir, modulePath string) error {
	content := fmt.Sprintf(`module %s

go 1.21

require (
	github.com/go-chi/chi/v5 v5.0.10
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.16.0
)
`, modulePath)

	return os.WriteFile(filepath.Join(targetDir, "go.mod"), []byte(content), 0644)
}

func createBasicFiles(targetDir string, data templateData) error {
	// README.md
	readmeContent := fmt.Sprintf(`# %s

%s

## Getting Started

1. Define your resources in pkg/resources/
2. Generate code: fabrica generate
3. Run the server: go run cmd/server/main.go

## Configuration

The server supports configuration via:
- Command line flags
- Environment variables (%s_*)
- Configuration file (~/.%s.yaml)

## Features

%s

## Development

`+"```bash"+`
# Install dependencies
go mod tidy

# Run the server
go run cmd/server/main.go serve

# Run with custom config
go run cmd/server/main.go serve --config config.yaml
`+"```"+`
`, data.ProjectName, data.Description, strings.ToUpper(data.ProjectName), data.ProjectName, generateFeaturesText(data))

	if err := os.WriteFile(filepath.Join(targetDir, "README.md"), []byte(readmeContent), 0644); err != nil {
		return err
	}

	// .gitignore
	gitignoreContent := `# Binaries
bin/
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary, built with go test -c
*.test

# Output of the go coverage tool
*.out

# Go workspace file
go.work

# Data directories
data/
*.db

# Config files (may contain secrets)
*.yaml
*.yml
!example.yaml
!example.yml

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~
`

	return os.WriteFile(filepath.Join(targetDir, ".gitignore"), []byte(gitignoreContent), 0644)
}

func generateFeaturesText(data templateData) string {
	var features []string

	if data.WithAuth {
		features = append(features, "- 🔐 Authentication with TokenSmith")
	}
	if data.WithStorage {
		if data.StorageType == "ent" {
			features = append(features, fmt.Sprintf("- 💾 Database storage (%s)", data.DBDriver))
		} else {
			features = append(features, "- 💾 File-based storage")
		}
	}
	if data.WithHSM {
		features = append(features, "- 🔒 HSM support for cryptographic operations")
	}
	if data.WithLegacyAPI {
		features = append(features, "- 🔄 Legacy API compatibility")
	}
	if data.WithMetrics {
		features = append(features, "- 📊 Prometheus metrics")
	}

	if len(features) == 0 {
		return "- Basic REST API server"
	}

	return strings.Join(features, "\n")
}

// createFabricaConfig creates a .fabrica.yaml configuration file to preserve project settings
func createFabricaConfig(targetDir string, opts *initOptions) error {
	// Extract project name from module path or target directory
	projectName := filepath.Base(targetDir)
	if targetDir == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		projectName = filepath.Base(cwd)
	}

	// Normalize database driver name (sqlite -> sqlite3 for Go driver compatibility)
	dbDriver := opts.dbDriver
	if dbDriver == "sqlite" {
		dbDriver = "sqlite3"
	}

	// Build configuration from options
	config := &FabricaConfig{
		Project: ProjectConfig{
			Name:        projectName,
			Module:      opts.modulePath,
			Description: opts.description,
			Created:     time.Now(),
		},
		Features: FeaturesConfig{
			Validation: ValidationConfig{
				Enabled: opts.validationMode != "disabled",
				Mode:    opts.validationMode,
			},
			Events: EventsConfig{
				Enabled: opts.withEvents,
				BusType: opts.eventBusType,
			},
			Conditional: ConditionalConfig{
				Enabled:       true, // Core feature always enabled
				ETagAlgorithm: "sha256",
			},
			Versioning: VersioningConfig{
				Enabled:        true, // Core feature always enabled
				Strategy:       opts.versionStrategy,
				DefaultVersion: "v1",
			},
			Auth: AuthConfig{
				Enabled: opts.withAuth,
			},
			Storage: StorageConfig{
				Enabled:  opts.withStorage,
				Type:     opts.storageType,
				DBDriver: dbDriver,
			},
			Metrics: MetricsConfig{
				Enabled: opts.withMetrics,
			},
			HSM: HSMConfig{
				Enabled: opts.withHSM,
			},
			LegacyAPI: LegacyAPIConfig{
				Enabled: opts.withLegacyAPI,
			},
		},
		Generation: GenerationConfig{
			Handlers:   true,
			Storage:    opts.withStorage,
			Client:     true,
			OpenAPI:    true,
			Events:     opts.withEvents,
			Middleware: true, // Core features always include middleware
		},
	}

	// Save configuration
	if err := SaveConfig(targetDir, config); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	fmt.Printf("  ├─ Created %s\n", ConfigFileName)

	return nil
}

// checkExistingProject checks if the directory already contains a Fabrica project
func checkExistingProject(dir string) error {
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

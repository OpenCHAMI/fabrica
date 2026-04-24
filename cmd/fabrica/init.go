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
	"text/template"
	"time"

	"github.com/openchami/fabrica/internal/constants"
	"github.com/openchami/fabrica/pkg/codegen"
	"github.com/spf13/cobra"
)

// initOptions holds all command-line flags and configuration for project initialization.
// These options control which features are enabled, API versioning settings,
// and storage backend configuration for the new Fabrica project.
type initOptions struct {
	interactive bool
	modulePath  string
	description string

	// Feature flags instead of modes
	withAuth    bool // Enable authentication
	withStorage bool // Enable storage backend
	withMetrics bool // Enable metrics/monitoring
	withVersion bool // Enable version command

	// New feature flags for core features
	validationMode string // strict, warn, disabled
	withEvents     bool   // Enable CloudEvents support
	eventBusType   string // memory, nats, kafka

	// API Versioning (hub/spoke)
	apiGroup       string   // e.g., "infra.example.io"
	storageVersion string   // Hub version (e.g., "v1")
	apiVersions    []string // All versions (e.g., ["v1alpha1", "v1"])

	// Reconciliation options
	withReconcile      bool // Enable reconciliation framework
	reconcileWorkers   int  // Number of reconciler workers
	reconcileRequeueMs int  // Default requeue delay in minutes

	// Storage options
	storageType string // file, ent
	dbDriver    string // postgres, mysql, sqlite
}

// templateData is passed to Go templates during project initialization.
// It contains all project metadata and feature flags needed to generate
// initial files (main.go, go.mod, README, etc.) with appropriate settings.
type templateData struct {
	ProjectName          string
	ModulePath           string
	Description          string
	WithAuth             bool
	WithStorage          bool
	WithMetrics          bool
	WithVersion          bool
	WithReconcile        bool
	WithEvents           bool
	StorageType          string
	DBDriver             string
	EventBusType         string
	ReconcileWorkers     int
	FabricaVersion       string
	GoVersion            string
	TokenSmithModulePath string
	TokenSmithVersion    string
	GeneratedAt          string
	CopyrightYear        int
	FeaturesText         string
}

// newInitCommand creates the 'fabrica init' cobra command.
// This command initializes a new Fabrica project with a .fabrica.yaml config,
// apis.yaml for versioning, project scaffolding, and initial directories.
//
// The command supports both non-interactive (flag-based) and interactive
// (wizard) modes. Default settings include file storage, strict validation,
// and a single v1 API version.
func newInitCommand() *cobra.Command {
	opts := &initOptions{
		withStorage:    true,           // Default to enabling storage
		withVersion:    true,           // Default to enabling version command
		storageType:    "file",         // Default to file storage
		dbDriver:       "sqlite",       // Default database
		validationMode: "strict",       // Default validation mode
		eventBusType:   "memory",       // Default event bus
		apiVersions:    []string{"v1"}, // Default hub version
	}

	cmd := &cobra.Command{
		Use:   "init [project-name]",
		Short: "Initialize a new Fabrica project",
		Long: `Initialize a new Fabrica project with configurable features.

Instead of complex modes, use feature flags to customize your project:
  --auth          Enable authentication with TokenSmith
  --storage       Enable persistent storage (file or database)
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

			// If group is specified but no versions, default to storage version
			if opts.apiGroup != "" && len(opts.apiVersions) == 0 {
				opts.apiVersions = []string{opts.storageVersion}
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
	cmd.Flags().BoolVar(&opts.withMetrics, "metrics", false, "Enable Prometheus metrics")
	cmd.Flags().BoolVar(&opts.withVersion, "version", true, "Enable version command")

	// Core feature configuration
	cmd.Flags().StringVar(&opts.validationMode, "validation-mode", "strict", "Validation mode: strict, warn, or disabled")
	cmd.Flags().BoolVar(&opts.withEvents, "events", false, "Enable CloudEvents support")
	cmd.Flags().StringVar(&opts.eventBusType, "events-bus", "memory", "Event bus type: memory, nats, or kafka")

	// API Versioning configuration
	cmd.Flags().StringVar(&opts.apiGroup, "group", "", "API group name (e.g., infra.example.io)")
	cmd.Flags().StringVar(&opts.storageVersion, "storage-version", "v1", "Hub (storage) version")
	cmd.Flags().StringSliceVar(&opts.apiVersions, "versions", []string{}, "API versions (comma-separated, e.g., v1alpha1,v1beta1,v1)")

	// Reconciliation configuration
	cmd.Flags().BoolVar(&opts.withReconcile, "reconcile", false, "Enable reconciliation framework")
	cmd.Flags().IntVar(&opts.reconcileWorkers, "reconcile-workers", 5, "Number of reconciler workers")
	cmd.Flags().IntVar(&opts.reconcileRequeueMs, "reconcile-requeue", 5, "Default requeue delay in minutes")

	// Storage options
	cmd.Flags().StringVar(&opts.storageType, "storage-type", "file", "Storage backend: file or ent")
	cmd.Flags().StringVar(&opts.dbDriver, "db", "sqlite", "Database driver for Ent: postgres, mysql, or sqlite")

	return cmd
}

// runInteractiveInit runs the interactive project initialization wizard.
// It prompts the user for project configuration through a series of questions,
// gathering project name, module path, feature selections, and storage preferences.
// This provides a more user-friendly experience than remembering all CLI flags.
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
	fmt.Printf("    Metrics: %s\n", map[bool]string{true: "enabled", false: "disabled"}[opts.withMetrics])

	fmt.Print("\nProceed? [Y/n]: ")
	input, _ = reader.ReadString('\n')
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "n") {
		fmt.Println("Cancelled.")
		return nil
	}

	return runInit(projectName, opts)
}

// runInit performs the actual project initialization.
// It creates the project directory structure, generates initial files from
// templates, writes .fabrica.yaml and apis.yaml configs, and displays next steps.
//
// The function supports initializing in the current directory (projectName=".")
// or creating a new directory. It checks for existing Fabrica projects to avoid
// accidental overwrites.
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
	apiConfig := DefaultAPIsConfig(opts.apiGroup, opts.storageVersion, opts.apiVersions)
	if group, err := apiConfig.primaryGroup(); err == nil {
		fmt.Printf("  1. Add resources with 'fabrica add resource <name>'\n")
		fmt.Printf("  2. Define types in apis/%s/<version>/*_types.go\n", group.Name)
	} else {
		fmt.Println("  1. Define your resources in apis/<group>/<version>/*_types.go")
	}
	fmt.Println("  3. Run 'fabrica generate' to generate code")
	fmt.Println("  4. Run 'go mod tidy' to update dependencies")
	fmt.Println("  5. Start development with 'go run ./cmd/server/'")
	fmt.Println()

	// Additional guidance for reconciliation
	if opts.withReconcile {
		fmt.Println("⚠️  Reconciliation Framework Enabled")
		fmt.Println()
		fmt.Println("To use reconciliation, you need to implement reconcilers:")
		fmt.Println()
		fmt.Println("  1. After running 'fabrica generate', a stub will be created in:")
		fmt.Println("     pkg/reconcilers/reconcilers_generated.go")
		fmt.Println()
		fmt.Println("  2. Create your custom reconciliation logic in pkg/reconcilers/custom.go:")
		fmt.Println()
		fmt.Println("     func MyResourceReconciler(ctx context.Context, req *reconcile.Request) error {")
		fmt.Println("       // Your reconciliation logic here")
		fmt.Println("       return nil")
		fmt.Println("     }")
		fmt.Println()
		fmt.Println("  3. Register it in pkg/reconcilers/reconcilers.go")
		fmt.Println("     (See Reconciliation Guide: docs/guides/reconciliation.md)")
		fmt.Println()
	}

	// Additional guidance for events
	if opts.withEvents && !opts.withReconcile {
		fmt.Println("💡 CloudEvents Enabled")
		fmt.Println()
		fmt.Println("Events are emitted for resource lifecycle (create, update, delete).")
		fmt.Println("See Events Guide: docs/guides/events.md")
		fmt.Println()
	}

	return nil
}

// createProjectStructure creates the directory tree and files for a new project.
// It generates main.go, runtime helper files, go.mod, README.md, .gitignore, and stub storage files
// from embedded templates, and writes both .fabrica.yaml and apis.yaml configs.
//
// The function creates:
//   - cmd/server/ directory with main.go
//   - internal/storage/ directory for storage implementations
//   - apis/<group>/<version>/ directories for each configured version
//   - Root config files (.fabrica.yaml, apis.yaml, go.mod, README.md, .gitignore)
func createProjectStructure(targetDir, projectName string, opts *initOptions) error {
	// Normalize database driver name (sqlite -> sqlite3 for Go driver compatibility)
	dbDriver := opts.dbDriver
	if dbDriver == "sqlite" {
		dbDriver = "sqlite3"
	}

	apiConfig := DefaultAPIsConfig(opts.apiGroup, opts.storageVersion, opts.apiVersions)
	group, err := apiConfig.primaryGroup()
	if err != nil {
		return err
	}
	now := time.Now().UTC()

	// Template data
	data := templateData{
		ProjectName:          projectName,
		ModulePath:           opts.modulePath,
		Description:          opts.description,
		WithAuth:             opts.withAuth,
		WithStorage:          opts.withStorage,
		WithMetrics:          opts.withMetrics,
		WithVersion:          opts.withVersion,
		WithReconcile:        opts.withReconcile,
		WithEvents:           opts.withEvents,
		StorageType:          opts.storageType,
		DBDriver:             dbDriver,
		EventBusType:         opts.eventBusType,
		ReconcileWorkers:     opts.reconcileWorkers,
		FabricaVersion:       version,
		GoVersion:            "1.21",
		TokenSmithModulePath: constants.TokenSmithModulePath,
		TokenSmithVersion:    constants.TokenSmithVersion,
		GeneratedAt:          now.Format(time.RFC3339),
		CopyrightYear:        now.Year(),
		FeaturesText:         "", // Will be populated later
	}
	if opts.withAuth {
		data.GoVersion = constants.TokenSmithGoVersion
	}

	// Generate features text
	data.FeaturesText = generateFeaturesText(data)

	// Create directories
	dirs := []string{
		"cmd/server",
		"internal/storage",
	}

	for _, version := range group.Versions {
		versionDir := filepath.Join("apis", group.Name, version)
		dirs = append(dirs, versionDir)
	}

	for _, dir := range dirs {
		path := filepath.Join(targetDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	// Generate main.go from template
	if err := generateFromTemplate("init/main.go.tmpl", filepath.Join(targetDir, "cmd/server/main.go"), data); err != nil {
		return err
	}

	// Generate runtime helpers from template
	if err := generateFromTemplate("init/runtime_helpers.go.tmpl", filepath.Join(targetDir, "cmd/server/runtime_helpers_generated.go"), data); err != nil {
		return err
	}

	// Generate auth helpers from template
	if err := generateFromTemplate("init/auth_helpers.go.tmpl", filepath.Join(targetDir, "cmd/server/auth_helpers_generated.go"), data); err != nil {
		return err
	}

	// Generate metrics helpers from template
	if err := generateFromTemplate("init/metrics_helpers.go.tmpl", filepath.Join(targetDir, "cmd/server/metrics_helpers_generated.go"), data); err != nil {
		return err
	}

	// Create go.mod from template
	if err := generateFromTemplate("init/go.mod.tmpl", filepath.Join(targetDir, "go.mod"), data); err != nil {
		return err
	}

	// Create README.md from template
	if err := generateFromTemplate("init/readme.md.tmpl", filepath.Join(targetDir, "README.md"), data); err != nil {
		return err
	}

	// Create .env template from template
	if err := generateFromTemplate("init/env.tmpl", filepath.Join(targetDir, ".env.template"), data); err != nil {
		return err
	}

	// Create .gitignore from template
	if err := generateFromTemplate("init/gitignore.tmpl", filepath.Join(targetDir, ".gitignore"), data); err != nil {
		return err
	}

	// Create Fabrica configuration file
	if err := createFabricaConfig(targetDir, opts); err != nil {
		return err
	}

	// Create apis.yaml to declare groups and versions
	if err := SaveAPIsConfig(targetDir, apiConfig); err != nil {
		return fmt.Errorf("failed to write %s: %w", APIsConfigFileName, err)
	}
	fmt.Printf("  ├─ Created %s (group %s, storage %s)\n", APIsConfigFileName, group.Name, group.StorageVersion)

	// Create stub storage files if storage is enabled
	if opts.withStorage {
		if err := createStubStorage(targetDir, data); err != nil {
			return err
		}
	}

	return nil
}

// generateFromTemplate executes a named template and writes the result to a file.
// It loads templates from the embedded filesystem (pkg/codegen/templates/),
// applies the provided data, and writes the rendered output.
//
// Template functions include toLower and toUpper for string manipulation.
func generateFromTemplate(templateName, outputPath string, data templateData) error {
	// Read template content from embedded filesystem
	tmplContent, err := codegen.GetEmbeddedTemplates().ReadFile("templates/" + templateName)
	if err != nil {
		return fmt.Errorf("template %s not found: %w", templateName, err)
	}

	// Template functions
	funcMap := template.FuncMap{
		"toLower": strings.ToLower,
		"toUpper": strings.ToUpper,
	}

	tmpl, err := template.New(templateName).Funcs(funcMap).Parse(string(tmplContent))
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

// generateFeaturesText creates a human-readable list of enabled features
// for inclusion in the generated README.md. It inspects the templateData
// and formats a bullet list of authentication, storage, and metrics features.
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
	if data.WithMetrics {
		features = append(features, "- 📊 Prometheus metrics")
	}

	if len(features) == 0 {
		return "- Basic REST API server"
	}

	return strings.Join(features, "\n")
}

// createFabricaConfig creates the .fabrica.yaml configuration file.
// It builds a FabricaConfig from initOptions and writes it to the target directory.
// The config preserves project settings for subsequent 'fabrica generate' invocations.
//
// Note: The Versioning field in FeaturesConfig is populated for backward
// compatibility but is deprecated. API versioning configuration should be
// edited in apis.yaml instead.
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
			Reconciliation: ReconciliationConfig{
				Enabled:      opts.withReconcile,
				WorkerCount:  opts.reconcileWorkers,
				RequeueDelay: opts.reconcileRequeueMs,
			},
			Security: SecurityConfig{
				AuthN: AuthNConfig{
					Enabled: opts.withAuth,
				},
				AuthZ: AuthZConfig{
					Enabled: false,
					Mode:    SecurityModeEnforce,
				},
			},
		},
		Generation: GenerationConfig{
			Handlers:       true,
			Storage:        opts.withStorage,
			Client:         true,
			OpenAPI:        true,
			Events:         opts.withEvents,
			Middleware:     true, // Core features always include middleware
			Reconciliation: opts.withReconcile,
		},
	}

	// Save configuration
	if err := SaveConfig(targetDir, config); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	fmt.Printf("  ├─ Created %s\n", ConfigFileName)

	return nil
}

// checkExistingProject verifies the directory doesn't already contain a Fabrica project.
// It looks for telltale files like cmd/server/main.go and pkg/resources/ to avoid
// accidentally overwriting an existing project.
//
// Returns an error if any Fabrica-specific files are found.
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

// createStubStorage creates placeholder storage files to prevent import errors.
// These stubs allow the generated main.go to compile before 'fabrica generate'
// is run for the first time. The stubs are replaced by actual implementations
// during code generation.
//
// For file storage, creates a simple stub.go. For Ent storage, creates additional
// stub directories (ent/, ent/migrate/, ent/schema/) that Ent will populate.
func createStubStorage(targetDir string, data templateData) error {
	storageDir := filepath.Join(targetDir, "internal", "storage")

	// Create stub storage.go file
	var stubContent string
	switch data.StorageType {
	case "file":
		stubContent = `// Code generated by Fabrica. DO NOT EDIT manually.
// SPDX-FileCopyrightText: Copyright © 2025-2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT
//
// This is a stub file created during init to prevent import errors.
// It will be replaced when you run 'fabrica generate --storage'

package storage

// Placeholder to prevent import errors - will be replaced by generated code
`
	case "ent":
		stubContent = `// Code generated by Fabrica. DO NOT EDIT manually.
// SPDX-FileCopyrightText: Copyright © 2025-2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT
//
// This is a stub file created during init to prevent import errors.
// It will be replaced when you run 'fabrica generate --storage'

package storage

// Placeholder to prevent import errors - will be replaced by generated code
`
		// For Ent storage, also create stub ent packages that main.go imports
		entDir := filepath.Join(storageDir, "ent")
		if err := os.MkdirAll(entDir, 0755); err != nil {
			return fmt.Errorf("failed to create ent directory: %w", err)
		}

		entStubContent := `// Code generated by Fabrica. DO NOT EDIT manually.
// SPDX-FileCopyrightText: Copyright © 2025-2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT
//
// This is a stub file created during init to prevent import errors.
// It will be replaced when Ent generates the real schema code.

package ent

// Placeholder to prevent import errors - will be replaced by Ent-generated code
`
		if err := os.WriteFile(filepath.Join(entDir, "stub.go"), []byte(entStubContent), 0644); err != nil {
			return fmt.Errorf("failed to create ent stub file: %w", err)
		}

		// Create stub migrate package
		migrateDir := filepath.Join(entDir, "migrate")
		if err := os.MkdirAll(migrateDir, 0755); err != nil {
			return fmt.Errorf("failed to create ent/migrate directory: %w", err)
		}

		migrateStubContent := `// Code generated by Fabrica. DO NOT EDIT manually.
// SPDX-FileCopyrightText: Copyright © 2025-2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT
//
// This is a stub file created during init to prevent import errors.
// It will be replaced when Ent generates the real migration code.

package migrate

// Placeholder to prevent import errors - will be replaced by Ent-generated code
`
		if err := os.WriteFile(filepath.Join(migrateDir, "stub.go"), []byte(migrateStubContent), 0644); err != nil {
			return fmt.Errorf("failed to create ent/migrate stub file: %w", err)
		}

		// Create stub schema sub-packages that Ent generates
		// These are created when storage layer uses Ent schemas
		schemaPackages := []string{"annotation", "label", "resource"}
		for _, pkg := range schemaPackages {
			pkgDir := filepath.Join(entDir, pkg)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				return fmt.Errorf("failed to create ent/%s directory: %w", pkg, err)
			}

			pkgStubContent := fmt.Sprintf(`// Code generated by Fabrica. DO NOT EDIT manually.
// SPDX-FileCopyrightText: Copyright © 2025-2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT
//
// This is a stub file created during init to prevent import errors.
// It will be replaced when Ent generates the real schema code.

package %s

// Placeholder to prevent import errors - will be replaced by Ent-generated code
`, pkg)
			if err := os.WriteFile(filepath.Join(pkgDir, "stub.go"), []byte(pkgStubContent), 0644); err != nil {
				return fmt.Errorf("failed to create ent/%s stub file: %w", pkg, err)
			}
		}
	}

	if err := os.WriteFile(filepath.Join(storageDir, "storage.go"), []byte(stubContent), 0644); err != nil {
		return fmt.Errorf("failed to create stub storage file: %w", err)
	}

	return nil
}

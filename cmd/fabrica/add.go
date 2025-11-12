// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type addOptions struct {
	withValidation bool
	withStatus     bool
	withVersioning bool
	packageName    string
	version        string // Target API version for versioned projects
	force          bool   // Force adding to non-alpha version
}

func newAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add resources or versions to your project",
		Long: `Add new resources or API versions to your Fabrica project.

Subcommands:
  resource  Add a new resource type
  version   Add a new API version

Examples:
  fabrica add resource Device --version v1alpha1
  fabrica add version v1beta2
`,
	}

	// Add subcommands
	cmd.AddCommand(newAddResourceCommand())
	cmd.AddCommand(newAddVersionCommand())

	return cmd
}

func newAddResourceCommand() *cobra.Command {
	opts := &addOptions{}

	cmd := &cobra.Command{
		Use:   "resource [name]",
		Short: "Add a new resource to your project",
		Long: `Add a new resource definition to your project.

This creates:
  - Resource definition file
  - Spec and Status structs
  - Optional validation
  - Registration code

Example:
  fabrica add resource Device --version v1alpha1
  fabrica add resource Product --version v1beta1 --with-validation
`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resourceName := args[0]
			return runAddResource(resourceName, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.withValidation, "with-validation", true, "Include validation tags")
	cmd.Flags().BoolVar(&opts.withStatus, "with-status", true, "Include Status struct")
	cmd.Flags().BoolVar(&opts.withVersioning, "with-versioning", false, "Enable per-resource spec versioning (snapshots). Status is never versioned.")
	cmd.Flags().StringVar(&opts.packageName, "package", "", "Package name (defaults to lowercase resource name)")
	cmd.Flags().StringVar(&opts.version, "version", "", "API version (required for versioned projects, e.g., v1alpha1)")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Force adding to non-alpha version")

	return cmd
}

// isFabricaProject checks if the current directory is a fabrica project
func isFabricaProject() bool {
	_, err := os.Stat(ConfigFileName)
	return err == nil
}

func runAddResource(resourceName string, opts *addOptions) error {
	// Check if we're in a fabrica project directory
	if !isFabricaProject() {
		fmt.Println("⚠️  Warning: This doesn't appear to be a fabrica project directory.")
		fmt.Println("Expected to find .fabrica.yaml in the current directory.")
		fmt.Print("\nAre you sure you want to continue? (y/N): ")

		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			return fmt.Errorf("operation cancelled")
		}
		fmt.Println()
	}

	// Load config to determine if this is a versioned project
	config, err := LoadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine target version and directory
	var targetDir string
	var isVersioned bool

	if config.Features.Versioning.Enabled && len(config.Features.Versioning.Versions) > 0 {
		isVersioned = true

		// Version is required for versioned projects
		if opts.version == "" {
			// Auto-select first alpha version
			for _, v := range config.Features.Versioning.Versions {
				if strings.Contains(v, "alpha") {
					opts.version = v
					fmt.Printf("No version specified, using first alpha version: %s\n", opts.version)
					break
				}
			}

			// If no alpha version, require explicit version with --force
			if opts.version == "" {
				return fmt.Errorf("no --version specified and no alpha version found.\nPlease specify a version with --version (use --force to add to stable version)")
			}
		} else {
			// Validate version exists in config
			found := false
			for _, v := range config.Features.Versioning.Versions {
				if v == opts.version {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("version %s not found in .fabrica.yaml (available: %v)", opts.version, config.Features.Versioning.Versions)
			}

			// Check if adding to non-alpha without --force
			if !strings.Contains(opts.version, "alpha") && !opts.force {
				return fmt.Errorf("adding resource to non-alpha version %s requires --force flag", opts.version)
			}
		}

		targetDir = filepath.Join("apis", config.Features.Versioning.Group, opts.version)
	} else {
		// Legacy mode: pkg/resources/
		isVersioned = false
		if opts.packageName == "" {
			opts.packageName = strings.ToLower(resourceName)
		}
		targetDir = filepath.Join("pkg", "resources", opts.packageName)
	}

	fmt.Printf("📦 Adding resource %s", resourceName)
	if isVersioned {
		fmt.Printf(" to %s/%s...\n", config.Features.Versioning.Group, opts.version)
	} else {
		fmt.Println("...")
	}

	// Create directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate resource file
	var resourceFile string
	if isVersioned {
		resourceFile = filepath.Join(targetDir, strings.ToLower(resourceName)+"_types.go")
	} else {
		resourceFile = filepath.Join(targetDir, opts.packageName+".go")
	}

	if err := generateResourceFile(resourceFile, resourceName, isVersioned, opts); err != nil {
		return err
	}

	// Update config to add resource to versioning.resources list
	if isVersioned {
		// Check if resource already in list
		found := false
		for _, r := range config.Features.Versioning.Resources {
			if r == resourceName {
				found = true
				break
			}
		}
		if !found {
			config.Features.Versioning.Resources = append(config.Features.Versioning.Resources, resourceName)
			if err := SaveConfig("", config); err != nil {
				return fmt.Errorf("failed to update config: %w", err)
			}
			fmt.Printf("  ✓ Added %s to .fabrica.yaml\n", resourceName)
		}
	}

	fmt.Println()
	fmt.Println("✅ Resource added successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit %s to customize your resource\n", resourceFile)
	if isVersioned {
		fmt.Printf("  2. Add to other versions with 'fabrica add version <new-version>'\n")
		fmt.Println("  3. Run 'fabrica generate' to create handlers")
	} else {
		fmt.Println("  2. Run 'fabrica generate' to create handlers")
		fmt.Println("  3. Implement custom business logic in handlers")
	}
	fmt.Println()

	return nil
}

func generateResourceFile(filePath, resourceName string, isVersioned bool, opts *addOptions) error {
	var packageName string
	if isVersioned {
		// Use version as package name (e.g., v1alpha1)
		packageName = opts.version
	} else {
		packageName = opts.packageName
	}

	content := fmt.Sprintf(`// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package %s

import (
	"context"`, packageName)

	if isVersioned {
		// Versioned types use flattened envelope
		content += `
	"github.com/openchami/fabrica/pkg/fabrica"
)

// ` + resourceName + ` represents a ` + strings.ToLower(resourceName) + ` resource
type ` + resourceName + ` struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       ` + resourceName + `Spec   ` + "`json:\"spec\""

		if opts.withValidation {
			content += ` validate:"required"`
		}
		content += "`\n"

		if opts.withStatus {
			content += fmt.Sprintf(`	Status     %sStatus `+"`json:\"status,omitempty\"`\n", resourceName)
		}
		content += `}

`
	} else {
		// Legacy: embedded resource.Resource
		content += `
	"github.com/openchami/fabrica/pkg/resource"
)

// ` + resourceName + ` represents a ` + resourceName + ` resource
type ` + resourceName + ` struct {
	resource.Resource
	Spec   ` + resourceName + `Spec   ` + "`json:\"spec\""

		if opts.withValidation {
			content += ` validate:"required"`
		}

		content += "`\n"

		if opts.withStatus {
			content += fmt.Sprintf(`	Status %sStatus `+"`json:\"status,omitempty\"`\n", resourceName)
		}

		content += `}

`
	}

	// Spec struct
	content += fmt.Sprintf(`// %sSpec defines the desired state of %s
type %sSpec struct {`, resourceName, resourceName, resourceName)

	if opts.withValidation {
		content += `
	Description string ` + "`json:\"description,omitempty\" validate:\"max=200\"`"
	} else {
		content += `
	Description string ` + "`json:\"description,omitempty\"`"
	}

	content += `
	// Add your spec fields here
}
`

	// Status struct
	if opts.withStatus {
		content += fmt.Sprintf(`
// %sStatus defines the observed state of %s
type %sStatus struct {
	Phase      string `+"`json:\"phase,omitempty\"`"+`
	Message    string `+"`json:\"message,omitempty\"`"+`
	Ready      bool   `+"`json:\"ready\"`"+`
	// Add your status fields here
}
`, resourceName, resourceName, resourceName)
	}

	// Validation method
	if opts.withValidation {
		content += fmt.Sprintf(`
// Validate implements custom validation logic for %s
func (r *%s) Validate(ctx context.Context) error {
	// Add custom validation logic here
	// Example:
	// if r.Spec.Description == "forbidden" {
	//     return errors.New("description 'forbidden' is not allowed")
	// }

	return nil
}
`, resourceName, resourceName)
	}

	// GetKind, GetName, GetUID methods
	if isVersioned {
		// Flattened envelope
		content += `// GetKind returns the kind of the resource
func (r *` + resourceName + `) GetKind() string {
	return "` + resourceName + `"
}

// GetName returns the name of the resource
func (r *` + resourceName + `) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *` + resourceName + `) GetUID() string {
	return r.Metadata.UID
}
`
	} else {
		// Legacy: embedded resource
		content += `// GetKind returns the kind of the resource
func (r *` + resourceName + `) GetKind() string {
	return "` + resourceName + `"
}

// GetName returns the name of the resource
func (r *` + resourceName + `) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *` + resourceName + `) GetUID() string {
	return r.Metadata.UID
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("` + resourceName + `", "` + strings.ToLower(resourceName)[:3] + `")
}
`
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

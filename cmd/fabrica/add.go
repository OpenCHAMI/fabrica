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
	packageName    string
}

func newAddCommand() *cobra.Command {
	opts := &addOptions{}

	cmd := &cobra.Command{
		Use:   "add resource [name]",
		Short: "Add a new resource to your project",
		Long: `Add a new resource definition to your project.

This creates:
  - Resource definition file
  - Spec and Status structs
  - Optional validation
  - Registration code

Example:
  fabrica add resource Device
  fabrica add resource Product --with-validation
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] != "resource" {
				return fmt.Errorf("unknown resource type: %s (only 'resource' is supported)", args[0])
			}

			if len(args) < 2 {
				return fmt.Errorf("resource name required")
			}

			resourceName := args[1]
			return runAddResource(resourceName, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.withValidation, "with-validation", true, "Include validation tags")
	cmd.Flags().BoolVar(&opts.withStatus, "with-status", true, "Include Status struct")
	cmd.Flags().StringVar(&opts.packageName, "package", "", "Package name (defaults to lowercase resource name)")

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

	if opts.packageName == "" {
		opts.packageName = strings.ToLower(resourceName)
	}

	fmt.Printf("📦 Adding resource %s...\n", resourceName)

	// Create package directory
	pkgDir := filepath.Join("pkg", "resources", opts.packageName)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return fmt.Errorf("failed to create package directory: %w", err)
	}

	// Generate resource file
	resourceFile := filepath.Join(pkgDir, opts.packageName+".go")
	if err := generateResourceFile(resourceFile, resourceName, opts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✅ Resource added successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit %s to customize your resource\n", resourceFile)
	fmt.Println("  2. Run 'fabrica generate' to create handlers")
	fmt.Println("  3. Implement custom business logic in handlers")
	fmt.Println()

	return nil
}

func generateResourceFile(filePath, resourceName string, opts *addOptions) error {
	packageName := opts.packageName

	content := fmt.Sprintf(`// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package %s

import (
	"context"
	"github.com/alexlovelltroy/fabrica/pkg/resource"`, packageName)

	// Note: validation package is imported in the fabrica library
	// and used implicitly through struct tags

	content += `
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

	if opts.withValidation {
		content += fmt.Sprintf(`
// Validate implements custom validation logic for %s
func (r *%s) Validate(ctx context.Context) error {
	// Add custom validation logic here
	// Example:
	// if r.Spec.Name == "forbidden" {
	//     return errors.New("name 'forbidden' is not allowed")
	// }

	return nil
}
`, resourceName, resourceName)
	}

	content += fmt.Sprintf(`
func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("%s", "%s")
}
`, resourceName, strings.ToLower(resourceName)[:3])

	return os.WriteFile(filePath, []byte(content), 0644)
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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

	apisConfig, err := LoadAPIsConfig("")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("apis.yaml not found; run 'fabrica init' to create it")
		}
		return fmt.Errorf("failed to load apis.yaml: %w", err)
	}

	group, err := apisConfig.primaryGroup()
	if err != nil {
		return err
	}

	// Determine target version and directory
	isVersioned := true
	if opts.version == "" {
		opts.version = group.StorageVersion
		fmt.Printf("No version specified, using storage hub version: %s\n", opts.version)
	} else {
		found := false
		for _, v := range group.Versions {
			if v == opts.version {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("version %s not found in apis.yaml (available: %v)\n\nTo add a new version, run: fabrica add version %s", opts.version, group.Versions, opts.version)
		}

		if !strings.Contains(opts.version, "alpha") && !opts.force {
			return fmt.Errorf("adding resource to non-alpha version %s requires --force flag\n\nStable versions should not have new resources added after release.\nUse --force if you understand the implications, or consider adding to an alpha version first.", opts.version) //nolint:all
		}
	}

	targetDir := filepath.Join("apis", group.Name, opts.version)

	fmt.Printf("📦 Adding resource %s", resourceName)
	if isVersioned {
		fmt.Printf(" to %s/%s...\n", group.Name, opts.version)
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

	if err := generateResourceFile(resourceFile, resourceName, isVersioned, opts, config.Project.Module, group.StorageVersion, group.Name); err != nil {
		return err
	}

	// Update apis.yaml to include the resource
	if isVersioned {
		apisConfig.addResource(resourceName)
		if err := SaveAPIsConfig("", apisConfig); err != nil {
			return fmt.Errorf("failed to update apis.yaml: %w", err)
		}
		fmt.Printf("  ✓ Added %s to apis.yaml\n", resourceName)
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

func generateResourceFile(filePath, resourceName string, isVersioned bool, opts *addOptions, modulePath, hubVersion, groupName string) error {
	if len(resourceName) > 0 {
        r := []rune(resourceName)
        r[0] = unicode.ToUpper(r[0])
        resourceName = string(r)
    }
	
	var packageName string
	if isVersioned {
		// Use version as package name (e.g., v1alpha1)
		packageName = opts.version
	} else {
		packageName = opts.packageName
	}

	// Determine if this is the hub version (storage version)
	isHub := isVersioned && hubVersion != "" && opts.version == hubVersion

	content := fmt.Sprintf(`// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package %s

import (
	"context"`, packageName)

	if isVersioned {
		// Versioned types use flattened envelope
		content += `
	"github.com/openchami/fabrica/pkg/fabrica"`

		// Add hub package import for spoke versions (for conversions)
		if !isHub && hubVersion != "" && groupName != "" && modulePath != "" {
			hubPackage := strings.ReplaceAll(hubVersion, ".", "")

			content += `
	` + hubPackage + ` "` + modulePath + `/apis/` + groupName + `/` + hubVersion + `"`
		}

		content += `
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
	`, resourceName, resourceName, resourceName)

		if opts.withVersioning {
			content += `	// Version is the current spec version identifier (server-managed)
	Version   string ` + "`json:\"version,omitempty\"`" + `
`
		}

		content += `	// Add your status fields here
}
`
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

		// Add IsHub marker for hub/storage version
		if isHub {
			content += `
// IsHub marks this as the hub/storage version
func (r *` + resourceName + `) IsHub() {}
`
		}

		// Add conversion stubs for non-hub versions (spokes)
		if !isHub && hubVersion != "" && groupName != "" {
			hubPackage := strings.ReplaceAll(hubVersion, ".", "")

			content += `
// ConvertTo converts this ` + packageName + ` ` + resourceName + ` to the hub version (` + hubVersion + `)
func (src *` + resourceName + `) ConvertTo(dstRaw interface{}) error {
	dst := dstRaw.(*` + hubPackage + `.` + resourceName + `)

	// TODO: Implement conversion logic from ` + packageName + ` to ` + hubVersion + `

	// Copy common fields
	dst.APIVersion = "` + groupName + `/` + hubVersion + `"
	dst.Kind = src.Kind
	dst.Metadata = src.Metadata

	// TODO: Convert Spec fields
	// Map fields from src.Spec to dst.Spec
	// Handle any field additions, removals, or transformations

	// TODO: Convert Status fields
	// Map fields from src.Status to dst.Status
	// Handle any field additions, removals, or transformations

	return nil
}

// ConvertFrom converts from the hub version (` + hubVersion + `) to this ` + packageName + ` ` + resourceName + `
func (dst *` + resourceName + `) ConvertFrom(srcRaw interface{}) error {
	src := srcRaw.(*` + hubPackage + `.` + resourceName + `)
	_ = src

	// TODO: Implement conversion logic from ` + hubVersion + ` to ` + packageName + `

	// Copy common fields
		dst.APIVersion = "` + groupName + `/` + packageName + `"
	// TODO: Convert Spec fields
	// Map fields from src.Spec to dst.Spec
	// Handle any field additions, removals, or transformations
	// Drop fields that don't exist in ` + packageName + `

	// TODO: Convert Status fields
	// Map fields from src.Status to dst.Status
	// Handle any field additions, removals, or transformations
	// Drop fields that don't exist in ` + packageName + `

	return nil
}
`
		}

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

	// Add a marker comment for per-resource versioning if enabled.
	// The generator will detect this and enable versioning templates.
	if opts.withVersioning {
		content = "// +fabrica:resource-versioning=enabled\n" + content
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

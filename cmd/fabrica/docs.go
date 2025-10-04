// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type docsOptions struct {
	topic  string
	level  string
	format string
}

func newDocsCommand() *cobra.Command {
	opts := &docsOptions{}

	cmd := &cobra.Command{
		Use:   "docs [topic]",
		Short: "Generate documentation with tiered complexity",
		Long: `Generate documentation organized by learning level.

Topics:
  getting-started  - Introduction and quick start
  validation       - Resource validation
  storage          - Storage backends
  events           - Event system
  reconciliation   - Reconciliation patterns
  versioning       - Multi-version APIs
  all              - Generate all documentation

Levels:
  beginner     - Essential concepts only
  intermediate - Common patterns and features
  advanced     - Complete reference

Examples:
  fabrica docs getting-started
  fabrica docs validation --level=beginner
  fabrica docs all --level=intermediate
`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			opts.topic = args[0]
			return runGenerateDocs(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.level, "level", "l", "all", "Documentation level: beginner, intermediate, advanced, all")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "markdown", "Output format: markdown, html")

	return cmd
}

func runGenerateDocs(opts *docsOptions) error {
	fmt.Printf("📖 Generating %s documentation...\n", opts.topic)

	docsDir := "docs"
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("failed to create docs directory: %w", err)
	}

	if opts.topic == "all" {
		topics := []string{"getting-started", "validation", "storage", "events", "reconciliation", "versioning"}
		for _, topic := range topics {
			if err := generateTopicDocs(docsDir, topic, opts.level); err != nil {
				return err
			}
		}
	} else {
		if err := generateTopicDocs(docsDir, opts.topic, opts.level); err != nil {
			return err
		}
	}

	// Generate index
	if err := generateDocsIndex(docsDir, opts.level); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✅ Documentation generated successfully!")
	fmt.Printf("  📁 %s/\n", docsDir)
	fmt.Println()
	fmt.Println("View documentation:")
	fmt.Printf("  open %s/index.md\n", docsDir)

	return nil
}

func generateTopicDocs(docsDir, topic, level string) error {
	var content string

	switch topic {
	case "getting-started":
		content = generateGettingStartedDocs(level)
	case "validation":
		content = generateValidationDocs(level)
	case "storage":
		content = generateStorageDocs(level)
	case "events":
		content = generateEventsDocs(level)
	case "reconciliation":
		content = generateReconciliationDocs(level)
	case "versioning":
		content = generateVersioningDocs(level)
	default:
		return fmt.Errorf("unknown topic: %s", topic)
	}

	filename := fmt.Sprintf("%s.md", topic)
	if level != "all" {
		filename = fmt.Sprintf("%s-%s.md", topic, level)
	}

	path := filepath.Join(docsDir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

func generateGettingStartedDocs(level string) string {
	beginner := `# Getting Started with Fabrica (Beginner)

## What is Fabrica?

Fabrica helps you build REST APIs quickly. Think of it as a code generator that creates:
- API endpoints for your data
- Database storage
- API documentation

## 5-Minute Quick Start

### 1. Create a Project

` + "```bash" + `
fabrica init myapi --mode=simple
cd myapi
` + "```" + `

### 2. Add a Resource

` + "```bash" + `
fabrica add resource Product
` + "```" + `

### 3. Generate Code

` + "```bash" + `
fabrica generate
` + "```" + `

### 4. Run

` + "```bash" + `
go run cmd/server/main.go
` + "```" + `

That's it! Your API is running on http://localhost:8080

## Next Steps

- [Add Validation](validation-beginner.md)
- [Customize Your Resource](resources-beginner.md)
- [Deploy Your API](deployment-beginner.md)
`

	if level == "beginner" {
		return beginner
	}

	// Return combined docs for "all" level
	return beginner + "\n\n" + `
## Intermediate Concepts

Once you're comfortable with the basics, explore:

- Resource metadata (labels, annotations)
- Event-driven patterns
- Storage backends
- Multi-version APIs

See [Getting Started (Intermediate)](getting-started-intermediate.md) for more.
`
}

func generateValidationDocs(level string) string {
	beginner := `# Validation (Beginner)

## Why Validate?

Validation ensures your API receives correct data before processing it.

## Basic Validation

Add validation tags to your structs:

` + "```go" + `
type Product struct {
    Name  string  ` + "`json:\"name\" validate:\"required,min=3\"`" + `
    Price float64 ` + "`json:\"price\" validate:\"required,gt=0\"`" + `
}
` + "```" + `

## Common Tags

- ` + "`required`" + ` - Field must be present
- ` + "`min=N`" + ` - Minimum length/value
- ` + "`max=N`" + ` - Maximum length/value
- ` + "`email`" + ` - Valid email format
- ` + "`url`" + ` - Valid URL format

## Using Validation

` + "```go" + `
if err := validation.ValidateResource(&product); err != nil {
    // Handle error
}
` + "```" + `

## Example

See ` + "`examples/validation-beginner/`" + ` for a complete example.
`

	if level == "beginner" {
		return beginner
	}

	return beginner // Add intermediate/advanced sections as needed
}

func generateStorageDocs(level string) string {
	return `# Storage (` + level + `)

Documentation coming soon.
`
}

func generateEventsDocs(level string) string {
	return `# Events (` + level + `)

Documentation coming soon.
`
}

func generateReconciliationDocs(level string) string {
	return `# Reconciliation (` + level + `)

Documentation coming soon.
`
}

func generateVersioningDocs(level string) string {
	return `# Versioning (` + level + `)

Documentation coming soon.
`
}

func generateDocsIndex(docsDir, _ string) error {
	content := `# Fabrica Documentation

Welcome to the Fabrica documentation!

## Getting Started

- [Quick Start](getting-started.md)
- [Installation](installation.md)

## Core Concepts

- [Resources](resources.md)
- [Validation](validation.md)
- [Storage](storage.md)

## Advanced Topics

- [Events](events.md)
- [Reconciliation](reconciliation.md)
- [Multi-Version APIs](versioning.md)

## Examples

See the ` + "`examples/`" + ` directory for complete working examples.

## API Reference

- [Resource API](api/resources.md)
- [Storage API](api/storage.md)
- [Validation API](api/validation.md)
`

	path := filepath.Join(docsDir, "index.md")
	return os.WriteFile(path, []byte(content), 0644)
}

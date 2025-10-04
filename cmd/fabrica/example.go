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

type exampleOptions struct {
	level    string // beginner, intermediate, advanced
	feature  string // validation, patch, events, reconciliation, etc.
	withDocs bool
}

func newExampleCommand() *cobra.Command {
	opts := &exampleOptions{}

	cmd := &cobra.Command{
		Use:   "example [name]",
		Short: "Generate example code with progressive disclosure",
		Long: `Generate example code organized by complexity level.

Levels:
  beginner     - Simple examples, minimal concepts
  intermediate - Common patterns, standard features
  advanced     - Complex scenarios, all features

Features:
  validation    - Validation examples
  patch         - PATCH operations
  events        - Event-driven patterns
  reconciliation- Reconciliation loops
  storage       - Storage backends
  versioning    - Multi-version APIs

Examples:
  fabrica example validation --level=beginner
  fabrica example patch --level=intermediate
  fabrica example reconciliation --level=advanced
`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			opts.feature = args[0]
			return runGenerateExample(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.level, "level", "l", "intermediate", "Complexity level: beginner, intermediate, advanced")
	cmd.Flags().BoolVar(&opts.withDocs, "with-docs", true, "Include documentation with example")

	return cmd
}

func runGenerateExample(opts *exampleOptions) error {
	fmt.Printf("📚 Generating %s example at %s level...\n", opts.feature, opts.level)

	exampleDir := filepath.Join("examples", opts.feature+"-"+opts.level)
	if err := os.MkdirAll(exampleDir, 0755); err != nil {
		return fmt.Errorf("failed to create example directory: %w", err)
	}

	// Generate example based on feature and level
	switch opts.feature {
	case "validation":
		return generateValidationExample(exampleDir, opts.level, opts.withDocs)
	case "patch":
		return generatePatchExample(exampleDir, opts.level, opts.withDocs)
	case "events":
		return generateEventsExample(exampleDir, opts.level, opts.withDocs)
	case "reconciliation":
		return generateReconciliationExample(exampleDir, opts.level, opts.withDocs)
	case "storage":
		return generateStorageExample(exampleDir, opts.level, opts.withDocs)
	case "versioning":
		return generateVersioningExample(exampleDir, opts.level, opts.withDocs)
	default:
		return fmt.Errorf("unknown feature: %s", opts.feature)
	}
}

func generateValidationExample(dir, level string, withDocs bool) error {
	var content string

	switch level {
	case "beginner":
		content = `package main

import (
	"encoding/json"
	"net/http"

	"github.com/alexlovelltroy/fabrica/pkg/validation"
)

// Simple product with basic validation
type Product struct {
	Name  string  ` + "`json:\"name\" validate:\"required,min=3\"`" + `
	Price float64 ` + "`json:\"price\" validate:\"required,gt=0\"`" + `
}

func main() {
	http.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		var product Product
		json.NewDecoder(r.Body).Decode(&product)

		// Validate
		if err := validation.ValidateResource(&product); err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(product)
	})

	http.ListenAndServe(":8080", nil)
}
`
	case "advanced":
		content = `package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/alexlovelltroy/fabrica/pkg/validation"
)

// Advanced resource with hybrid validation
type Order struct {
	ID       string  ` + "`json:\"id\"`" + `
	Product  string  ` + "`json:\"product\" validate:\"required,k8sname\"`" + `
	Quantity int     ` + "`json:\"quantity\" validate:\"required,min=1,max=1000\"`" + `
	Status   string  ` + "`json:\"status\" validate:\"required,oneof=pending confirmed shipped\"`" + `
	Total    float64 ` + "`json:\"total\" validate:\"required,gt=0\"`" + `
	Discount float64 ` + "`json:\"discount\" validate:\"gte=0\"`" + `
}

// Custom validation with business logic
func (o *Order) Validate(ctx context.Context) error {
	if o.Discount > o.Total {
		return errors.New("discount cannot exceed total")
	}

	if o.Status == "shipped" && o.Total == 0 {
		return errors.New("shipped orders must have been paid")
	}

	if o.Status == "confirmed" && !strings.HasPrefix(o.ID, "ORD-") {
		return errors.New("confirmed orders must have valid order ID")
	}

	return nil
}

func main() {
	http.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		var order Order
		json.NewDecoder(r.Body).Decode(&order)

		// Hybrid validation: struct tags + custom logic
		if err := validation.ValidateWithContext(r.Context(), &order); err != nil {
			if validationErrs, ok := err.(validation.ValidationErrors); ok {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "Validation failed",
					"details": validationErrs.Errors,
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(order)
	})

	http.ListenAndServe(":8080", nil)
}
`
	default: // intermediate
		content = `package main

import (
	"encoding/json"
	"net/http"

	"github.com/alexlovelltroy/fabrica/pkg/validation"
)

// Device with Kubernetes-style naming
type Device struct {
	Name     string            ` + "`json:\"name\" validate:\"required,k8sname\"`" + `
	Type     string            ` + "`json:\"type\" validate:\"required,oneof=server switch router\"`" + `
	Status   string            ` + "`json:\"status\" validate:\"required,oneof=active inactive\"`" + `
	Tags     []string          ` + "`json:\"tags\" validate:\"dive,labelvalue\"`" + `
	Labels   map[string]string ` + "`json:\"labels\" validate:\"dive,keys,labelkey,endkeys,labelvalue\"`" + `
}

func main() {
	http.HandleFunc("/devices", func(w http.ResponseWriter, r *http.Request) {
		var device Device
		json.NewDecoder(r.Body).Decode(&device)

		// Validate with K8s validators
		if err := validation.ValidateResource(&device); err != nil {
			if validationErrs, ok := err.(validation.ValidationErrors); ok {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "Validation failed",
					"details": validationErrs.Errors,
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(device)
	})

	http.ListenAndServe(":8080", nil)
}
`
	}

	mainPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainPath, []byte(content), 0644); err != nil {
		return err
	}

	if withDocs {
		readme := fmt.Sprintf(`# Validation Example (%s Level)

This example demonstrates validation at the %s complexity level.

## Run

`+"```bash"+`
go run main.go
`+"```"+`

## Test

`+"```bash"+`
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Product","price":9.99}'
`+"```"+`
`, level, level)

		readmePath := filepath.Join(dir, "README.md")
		if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
			return err
		}
	}

	fmt.Println("✅ Example generated successfully!")
	fmt.Printf("  📁 %s/\n", dir)
	fmt.Printf("  📄 %s/main.go\n", dir)
	if withDocs {
		fmt.Printf("  📖 %s/README.md\n", dir)
	}

	return nil
}

// Placeholder implementations for other example types
func generatePatchExample(_ string, _ string, _ bool) error {
	fmt.Println("  ⚠️  PATCH examples coming soon")
	return nil
}

func generateEventsExample(_ string, _ string, _ bool) error {
	fmt.Println("  ⚠️  Events examples coming soon")
	return nil
}

func generateReconciliationExample(_ string, _ string, _ bool) error {
	fmt.Println("  ⚠️  Reconciliation examples coming soon")
	return nil
}

func generateStorageExample(_ string, _ string, _ bool) error {
	fmt.Println("  ⚠️  Storage examples coming soon")
	return nil
}

func generateVersioningExample(_ string, _ string, _ bool) error {
	fmt.Println("  ⚠️  Versioning examples coming soon")
	return nil
}

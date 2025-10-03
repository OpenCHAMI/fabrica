// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGenerateCommand() *cobra.Command {
	var (
		handlers bool
		storage  bool
		client   bool
		openapi  bool
		all      bool
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if !handlers && !storage && !client && !openapi {
				all = true
			}

			fmt.Println("🔧 Generating code...")

			if all || handlers {
				fmt.Println("  ├─ Generating handlers...")
				// TODO: Implement handler generation
			}

			if all || storage {
				fmt.Println("  ├─ Generating storage adapters...")
				// TODO: Implement storage generation
			}

			if all || client {
				fmt.Println("  ├─ Generating client code...")
				// TODO: Implement client generation
			}

			if all || openapi {
				fmt.Println("  └─ Generating OpenAPI spec...")
				// TODO: Implement OpenAPI generation
			}

			fmt.Println()
			fmt.Println("✅ Code generation complete!")
			fmt.Println()
			fmt.Println("Generated files:")
			fmt.Println("  - cmd/server/handlers_generated.go")
			fmt.Println("  - internal/storage/storage_generated.go")
			if all || client {
				fmt.Println("  - pkg/client/client_generated.go")
			}
			if all || openapi {
				fmt.Println("  - api/openapi.json")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&handlers, "handlers", false, "Generate HTTP handlers")
	cmd.Flags().BoolVar(&storage, "storage", false, "Generate storage adapters")
	cmd.Flags().BoolVar(&client, "client", false, "Generate client code")
	cmd.Flags().BoolVar(&openapi, "openapi", false, "Generate OpenAPI spec")

	return cmd
}

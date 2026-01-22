// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// This file shows practical patterns for using the generated storage functions.
// Copy patterns from this file into your generated handlers.

package storage

import (
	"context"
	"fmt"

	"your-module/internal/storage"
	"your-module/internal/storage/ent"
	"your-module/apis/infra.example.com/v1"
)

// Example 1: Query with Label Filtering
// ======================================
//
// Pattern: Use ListXXXByLabels to filter resources without loading all data
//
// Generated function signature:
//   func ListServersByLabels(ctx context.Context, labels map[string]string) ([]*v1.Server, error)
//
// Real-world use case: API endpoint for filtering
func ExampleQueryByLabels(ctx context.Context) error {
	// Parse labels from query parameters: ?env=prod&zone=us-east-1
	labels := map[string]string{
		"env":  "prod",
		"zone": "us-east-1",
	}

	// Query servers matching ALL labels
	// Generated SQL: SELECT * FROM resources WHERE kind='Server'
	//               AND (key='env' AND value='prod')
	//               AND (key='zone' AND value='us-east-1')
	servers, err := storage.ListServersByLabels(ctx, labels)
	if err != nil {
		return fmt.Errorf("failed to query servers: %w", err)
	}

	fmt.Printf("Found %d servers matching labels\n", len(servers))
	for _, srv := range servers {
		fmt.Printf("  - %s (%s)\n", srv.Metadata.Name, srv.Metadata.UID)
	}

	return nil
}

// Example 2: Atomic Multi-Resource Transaction
// =============================================
//
// Pattern: Use WithTx to ensure multiple resources are created/updated together
//
// Real-world use case: Create a server with its default configuration
func ExampleAtomicTransaction(ctx context.Context) error {
	// Atomically create server and config, or rollback both
	err := storage.WithTx(ctx, func(tx *ent.Tx) error {
		// Step 1: Create server
		server := &v1.Server{
			APIVersion: "v1",
			Kind:       "Server",
			Metadata: v1.Metadata{
				UID:  "srv-abc123",
				Name: "server-1",
				Labels: map[string]string{
					"env": "prod",
				},
			},
			Spec: v1.ServerSpec{
				Hostname:  "srv1.example.com",
				IPAddress: "10.0.1.10",
			},
		}

		// Create in transaction (not committed yet)
		serverResource, err := storage.ToEntResource(server)
		if err != nil {
			return fmt.Errorf("failed to prepare server: %w", err)
		}

		// Save to Ent using tx instead of entClient
		// entResource, err := tx.Resource.Create().
		//     SetUID(server.Metadata.UID).
		//     ... other fields ...
		//     Save(ctx)

		fmt.Printf("Created server: %s\n", server.Metadata.Name)

		// Step 2: Create config (if this fails, server creation rolls back)
		config := &v1.ServerConfig{
			APIVersion: "v1",
			Kind:       "ServerConfig",
			Metadata: v1.Metadata{
				UID:  "cfg-abc123",
				Name: "config-1",
				Labels: map[string]string{
					"server": "srv-abc123",
				},
			},
			Spec: v1.ServerConfigSpec{
				ServerUID: "srv-abc123",
				CPUs:      4,
				Memory:    16384,
			},
		}

		fmt.Printf("Created config: %s\n", config.Metadata.Name)

		return nil  // Commit both
	})

	if err != nil {
		return fmt.Errorf("transaction failed, all changes rolled back: %w", err)
	}

	fmt.Println("✓ Server and config created atomically")
	return nil
}

// Example 3: Bulk Update with Transaction
// ========================================
//
// Pattern: Update multiple resources in a single transaction
func ExampleBulkUpdate(ctx context.Context, serverUIDs []string, status string) error {
	count := 0

	err := storage.WithTx(ctx, func(tx *ent.Tx) error {
		for _, uid := range serverUIDs {
			// Load server
			srv, err := storage.GetServerByUID(ctx, uid)
			if err != nil {
				return fmt.Errorf("failed to load server %s: %w", uid, err)
			}

			// Update status
			srv.Status.Phase = status

			// Save updated server (using tx for atomicity)
			// In real code, marshal and update via tx.Resource.Update()
			fmt.Printf("Updated %s to %s\n", srv.Metadata.Name, status)
			count++
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("bulk update failed: %w", err)
	}

	fmt.Printf("✓ Updated %d servers atomically\n", count)
	return nil
}

// Example 4: Query with Pagination
// ================================
//
// Pattern: Use query builder for efficient pagination
func ExamplePagination(ctx context.Context, pageSize int, pageNum int) error {
	offset := pageSize * (pageNum - 1)

	// Query builder allows efficient pagination
	// Generated: QueryServers(ctx) returns *ent.ResourceQuery
	servers, err := storage.QueryServers(ctx).
		Limit(pageSize).
		Offset(offset).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query page %d: %w", pageNum, err)
	}

	fmt.Printf("Page %d (offset %d, limit %d): found %d servers\n",
		pageNum, offset, pageSize, len(servers))

	return nil
}

// Example 5: Complex Query with Multiple Filters
// =============================================
//
// Pattern: Build on generated query functions for advanced filtering
func ExampleComplexQuery(ctx context.Context) error {
	// Find active production servers sorted by creation date
	servers, err := storage.QueryServers(ctx).
		// Note: For more complex filtering, build query helpers
		// QueryServers returns *ent.ResourceQuery with access to:
		// - Where() for additional conditions
		// - Order() for sorting
		// - Limit/Offset for pagination
		// - WithLabels() for eager loading
		All(ctx)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	fmt.Printf("Found %d servers\n", len(servers))

	// Filter in application layer
	var activeServers []*v1.Server
	for _, srv := range servers {
		if srv.Status.Phase == "Active" && hasLabel(srv, "env", "prod") {
			activeServers = append(activeServers, srv)
		}
	}

	fmt.Printf("After filtering: %d active production servers\n", len(activeServers))
	return nil
}

// Example 6: Migration Pattern
// ===========================
//
// Pattern: Export resources, then import into another system
func ExampleMigrationPattern() {
	// Step 1: In old system, export resources
	// ./myapi export --format yaml --output ./migration-data/

	// Step 2: Transfer files to new system (scp, git, S3, etc.)

	// Step 3: In new system, import resources
	// ./myapi import --input ./migration-data/ --mode replace

	fmt.Println("Migration pattern:")
	fmt.Println("1. Export from old system: ./myapi export --output ./backup/")
	fmt.Println("2. Transfer files to new system")
	fmt.Println("3. Import in new system: ./myapi import --input ./backup/")
	fmt.Println("\nFor production, use replace mode to ensure consistency.")
}

// Helper functions

func hasLabel(srv *v1.Server, key, value string) bool {
	if srv.Metadata.Labels == nil {
		return false
	}
	return srv.Metadata.Labels[key] == value
}

// Example Handler Integration
// ============================
//
// This shows how to integrate patterns into HTTP handlers
type ServerHandler struct {
	// context passed from main.go
}

// ListServers handles GET /api/v1/servers?env=prod&zone=us-east-1
func (h *ServerHandler) ListServers(ctx context.Context, queryParams map[string]string) ([]*v1.Server, error) {
	// Extract label filters from query params
	labels := make(map[string]string)
	for key, value := range queryParams {
		if key != "page" && key != "limit" {
			labels[key] = value
		}
	}

	if len(labels) == 0 {
		// No filters, use generic query
		return storage.QueryServers(ctx).All(ctx)
	}

	// Exact label match
	return storage.ListServersByLabels(ctx, labels)
}

// CreateServerWithConfig handles POST /api/v1/servers with automatic config creation
func (h *ServerHandler) CreateServerWithConfig(ctx context.Context, server *v1.Server) error {
	return storage.WithTx(ctx, func(tx *ent.Tx) error {
		// Create server
		// Create default config
		// Both succeed or both fail
		return nil
	})
}

// Example 7: Export/Import for Backups and Migration
// ===================================================
//
// Pattern: Use generated export/import commands for data portability
//
// Commands generated in cmd/server/export.go and cmd/server/import.go
//
// Real-world use cases:
//   - Regular backups for disaster recovery
//   - Migrating data between dev/staging/prod
//   - Version controlling resource definitions
//   - Seeding test data

// Export all resources:
//   ./myapi export --format yaml --output ./backup
//
// Export specific types:
//   ./myapi export --kinds Server,Rack --output ./partial
//
// Import from backup:
//   ./myapi import --input ./backup
//
// Dry run to preview:
//   ./myapi import --input ./backup --dry-run
//
// Replace mode (delete all first):
//   ./myapi import --input ./backup --mode replace

// Implementation details:
//   - Commands use storage.Query{Resource}(ctx).All(ctx) for direct storage access
//   - Support JSON and YAML formats
//   - Import modes: upsert (default), replace (delete-first), skip (existing)
//   - Atomic operations via storage.WithTx() for safe imports
//   - Works offline without running HTTP server

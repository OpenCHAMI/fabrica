#!/bin/bash
# Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-License-Identifier: MIT

# Quick start script for the Ent Advanced example
# This script demonstrates the query builders, transactions, and export/import features

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
WORK_DIR="${WORK_DIR:-/tmp/fabrica-ent-advanced-demo}"

echo "=== Fabrica Ent Advanced Example ==="
echo "Demo directory: $WORK_DIR"
echo ""

# Cleanup on exit (optional, comment out to inspect files)
cleanup() {
    if [ "$KEEP_DEMO" != "1" ]; then
        echo "Cleaning up demo directory (set KEEP_DEMO=1 to keep it)"
        rm -rf "$WORK_DIR"
    else
        echo "Demo directory kept at: $WORK_DIR"
    fi
}
trap cleanup EXIT

mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

# Step 1: Initialize project
echo "Step 1: Initialize Fabrica project with Ent storage..."
fabrica init demo-api --storage-type=ent --db=sqlite
cd demo-api

# Use the local Fabrica source for code generation without modifying demo-api/go.mod
export FABRICA_SOURCE_PATH="$REPO_ROOT"

echo "✓ Project initialized"
echo ""

# Step 2: Add resources
echo "Step 2: Adding resources..."
fabrica add resource Server
fabrica add resource Node

echo "✓ Resources added"
echo ""

# Step 3: Generate code
echo "Step 3: Generating code with query builders and transactions..."
fabrica generate

echo "✓ Code generated"
echo ""

# Step 4: Show generated functions
echo "Step 4: Inspecting generated storage functions..."
echo ""

echo "Generated Ent storage files:"
ls -1 internal/storage/ent*.go 2>/dev/null | sed 's/^/  /' || echo "  No Ent files found"
echo ""

echo "Note: With v0.4.0+, Ent uses a unified adapter pattern."
echo "Query builders and transactions are in the generated storage layer."

# Step 5: Show build info
echo "Step 5: Build information..."
if command -v go &> /dev/null; then
    echo "Go version: $(go version)"
    echo "Project structure:"
    find . -name "*.go" -type f | grep -E "(ent_queries|ent_transactions|storage\.go)" | head -5
    echo ""
else
    echo "Go not found in PATH"
fi

# Step 6: Print next steps
echo "Step 6: Next steps for hands-on exploration..."
echo ""
echo "1. View generated query builders:"
echo "   cat internal/storage/ent_queries_generated.go"
echo ""
echo "2. View transaction support:"
echo "   cat internal/storage/ent_transactions_generated.go"
echo ""
echo "3. See example patterns:"
echo "   cat $SCRIPT_DIR/PATTERNS.md"
echo ""
echo "4. Read comprehensive documentation:"
echo "   cat $SCRIPT_DIR/README.md"
echo ""
echo "5. Try export/import (when server is running):"
echo "   go run ./cmd/server/ &"
echo "   go run ./cmd/server export --format yaml --output ./backup/"
echo "   go run ./cmd/server import --input ./backup/"
echo ""

echo "=== Demo Complete ==="
echo ""
echo "Key takeaways:"
echo "✓ Query builders: Type-safe, efficient filtering without loading all data"
echo "✓ Transactions: Atomic operations with automatic rollback on error"
echo "✓ Export/Import: CLI for backup, migration, and disaster recovery"
echo ""
echo "For detailed patterns and recipes, see PATTERNS.md in this directory"

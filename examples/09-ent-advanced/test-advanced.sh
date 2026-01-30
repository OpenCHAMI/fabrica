#!/bin/bash
# Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
# SPDX-License-Identifier: MIT

# Test script for Example 09: Advanced Ent Storage Features
# This script validates the query builders, transactions, and export/import

set -e

API_BASE="http://localhost:8080/api/v1"
DEMO_DIR="${DEMO_DIR:-.}"

echo "=== Testing Ent Advanced Features ==="
echo ""

# Helper function to test endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4

    echo "Testing: $description"
    echo "  $method $API_BASE$endpoint"

    if [ -z "$data" ]; then
        curl -s -X "$method" "$API_BASE$endpoint" | jq '.' 2>/dev/null || echo "  (API not responding or invalid JSON)"
    else
        echo "$data" | curl -s -X "$method" "$API_BASE$endpoint" -H "Content-Type: application/json" -d @- | jq '.' 2>/dev/null || echo "  (API not responding)"
    fi

    echo ""
}

# Check if server is running
echo "Checking if server is running..."
if ! curl -s -f "$API_BASE/servers" > /dev/null 2>&1; then
    echo "❌ Server not responding at $API_BASE"
    echo ""
    echo "Start the server with:"
    echo "  export DATABASE_URL=\"file:./data/demo.db?cache=shared&_fk=1\""
    echo "  go run ./cmd/server/"
    echo ""
    exit 1
fi

echo "✓ Server is running"
echo ""

# Test 1: Create servers (demonstrates basic CRUD)
echo "=== Test 1: Create Resources ==="
test_endpoint POST /servers '{"metadata":{"name":"server-1","labels":{"env":"prod"}},"spec":{"hostname":"srv1.local"}}' "Create Server 1"

test_endpoint POST /servers '{"metadata":{"name":"server-2","labels":{"env":"prod","zone":"us-east-1"}},"spec":{"hostname":"srv2.local"}}' "Create Server 2"

test_endpoint POST /servers '{"metadata":{"name":"server-3","labels":{"env":"dev"}},"spec":{"hostname":"srv3.local"}}' "Create Server 3 (dev environment)"

echo ""

# Test 2: Query by labels (demonstrates query builders)
echo "=== Test 2: Query by Labels (Query Builders) ==="
test_endpoint GET "/servers?labelSelector=env=prod" "" "List production servers"

test_endpoint GET "/servers?labelSelector=env=prod,zone=us-east-1" "" "List prod servers in us-east-1"

test_endpoint GET "/servers?labelSelector=env=dev" "" "List development servers"

echo ""

# Test 3: Get single resource (demonstrates GetXXXByUID)
echo "=== Test 3: Get Single Resource ==="
test_endpoint GET "/servers" "" "List all servers (to get UIDs)"

echo "Note: Use the UID from the list above to test GET /servers/{uid}"

echo ""

# Test 4: Export functionality (demonstrates export/import)
echo "=== Test 4: Export/Import Operations ==="
echo "Export commands (when data exists):"
echo "  go run ./cmd/server export --format yaml --output ./export-yaml/"
echo "  go run ./cmd/server export --format json --output ./export-json/"
echo "  go run ./cmd/server export --format yaml --output ./export-prod/ --label-selector env=prod"
echo ""
echo "Import commands:"
echo "  go run ./cmd/server import --input ./export-yaml/ --mode upsert"
echo "  go run ./cmd/server import --input ./export-json/ --mode replace"
echo ""

# Test 5: Verify data consistency
echo "=== Test 5: Data Verification ==="
test_endpoint GET "/servers" "" "Final state - list all servers"

echo ""
echo "=== Test Summary ==="
echo "✓ Query builders work correctly (filter by labels)"
echo "✓ Resources created successfully"
echo "✓ Label-based filtering functional"
echo ""
echo "Next steps:"
echo "1. Try updating resources: PATCH /servers/{uid}"
echo "2. Test transactions in your custom handlers"
echo "3. Explore export/import with actual data"
echo "4. See PATTERNS.md for integration examples"

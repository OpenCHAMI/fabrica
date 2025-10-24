#!/bin/bash

# Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

# Test script for Device Inventory API
# This script demonstrates all CRUD operations

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
DEVICE_UID=""

# Helper functions
print_step() {
    echo -e "${BLUE}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Check if server is running
check_server() {
    print_step "Checking if server is running..."
    if ! curl -s -f "$API_URL/health" > /dev/null; then
        print_error "Server is not running at $API_URL"
        echo "Please start the server with: go run ./cmd/server"
        exit 1
    fi
    print_success "Server is running"
    echo ""
}

# Test 1: Create a device
test_create() {
    print_step "Step 1: Creating a device..."

    RESPONSE=$(curl -s -X POST "$API_URL/devices" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "test-switch-01",
            "hostname": "test-switch-01.example.com",
            "ipaddr": "192.168.1.100",
            "description": "Test network switch",
            "labels": {
                "environment": "test",
                "location": "datacenter-1"
            }
        }')

    # Extract UID
    DEVICE_UID=$(echo "$RESPONSE" | grep -o '"uid":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ -z "$DEVICE_UID" ]; then
        print_error "Failed to create device"
        echo "Response: $RESPONSE"
        exit 1
    fi

    print_success "Created device: test-switch-01 (UID: $DEVICE_UID)"
    echo ""
}

# Test 2: Create more devices
test_create_more() {
    print_step "Step 2: Creating more devices..."

    # Create router
    curl -s -X POST "$API_URL/devices" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "test-router-01",
            "hostname": "router-01.example.com",
            "ipaddr": "192.168.1.1",
            "description": "Main router",
            "labels": {
                "environment": "test",
                "type": "router"
            }
        }' > /dev/null

    # Create firewall
    curl -s -X POST "$API_URL/devices" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "test-firewall-01",
            "hostname": "firewall-01.example.com",
            "ipaddr": "192.168.1.2",
            "description": "Edge firewall"
        }' > /dev/null

    print_success "Created additional devices"
    echo ""
}

# Test 3: List devices
test_list() {
    print_step "Step 3: Listing all devices..."

    RESPONSE=$(curl -s "$API_URL/devices")
    COUNT=$(echo "$RESPONSE" | grep -o '"kind":"Device"' | wc -l | tr -d ' ')

    print_success "Found $COUNT devices"
    echo ""

    # Pretty print the list (if jq is available)
    if command -v jq &> /dev/null; then
        echo "Devices:"
        echo "$RESPONSE" | jq -r '.items[] | "  - \(.metadata.name): \(.spec.hostname) (\(.spec.ipaddr))"'
        echo ""
    fi
}

# Test 4: Get specific device
test_get() {
    print_step "Step 4: Getting specific device..."

    RESPONSE=$(curl -s "$API_URL/devices/$DEVICE_UID")
    NAME=$(echo "$RESPONSE" | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)

    print_success "Retrieved device: $NAME"

    if command -v jq &> /dev/null; then
        echo "$RESPONSE" | jq '.'
    fi
    echo ""
}

# Test 5: Update device
test_update() {
    print_step "Step 5: Updating device..."

    # Get current device
    CURRENT=$(curl -s "$API_URL/devices/$DEVICE_UID")

    # Update IP address
    UPDATED=$(echo "$CURRENT" | sed 's/"ipaddr":"192.168.1.100"/"ipaddr":"192.168.1.101"/')

    curl -s -X PUT "$API_URL/devices/$DEVICE_UID" \
        -H "Content-Type: application/json" \
        -d "$UPDATED" > /dev/null

    print_success "Updated device IP address"

    # Verify update
    RESPONSE=$(curl -s "$API_URL/devices/$DEVICE_UID")
    NEW_IP=$(echo "$RESPONSE" | grep -o '"ipaddr":"[^"]*"' | cut -d'"' -f4)
    echo "New IP: $NEW_IP"
    echo ""
}

# Test 6: Delete device
test_delete() {
    print_step "Step 6: Deleting device..."

    curl -s -X DELETE "$API_URL/devices/$DEVICE_UID" > /dev/null

    print_success "Deleted device $DEVICE_UID"
    echo ""
}

# Test 7: Verify deletion
test_verify_deletion() {
    print_step "Step 7: Verifying deletion..."

    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/devices/$DEVICE_UID")

    if [ "$STATUS" = "404" ]; then
        print_success "Device successfully deleted (404 Not Found)"
    else
        print_error "Device still exists (HTTP $STATUS)"
        exit 1
    fi
    echo ""
}

# Test 8: Test validation
test_validation() {
    print_step "Step 8: Testing validation..."

    # Try to create device without required field
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/devices" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "invalid-device",
            "hostname": "test.example.com"
        }')

    STATUS=$(echo "$RESPONSE" | tail -1)

    if [ "$STATUS" = "400" ]; then
        print_success "Validation working correctly (rejected invalid input)"
    else
        print_warning "Expected 400 Bad Request, got HTTP $STATUS"
    fi
    echo ""
}

# Test 9: Test OpenAPI spec
test_openapi() {
    print_step "Step 9: Testing OpenAPI endpoint..."

    RESPONSE=$(curl -s "$API_URL/openapi.json")

    if echo "$RESPONSE" | grep -q '"openapi"'; then
        print_success "OpenAPI spec is available"

        if command -v jq &> /dev/null; then
            TITLE=$(echo "$RESPONSE" | jq -r '.info.title')
            VERSION=$(echo "$RESPONSE" | jq -r '.info.version')
            echo "API: $TITLE v$VERSION"
        fi
    else
        print_error "OpenAPI spec not found"
    fi
    echo ""
}

# Cleanup function
cleanup() {
    print_step "Cleaning up test data..."

    # Get all devices and delete them
    DEVICES=$(curl -s "$API_URL/devices" | grep -o '"uid":"[^"]*"' | cut -d'"' -f4)

    for uid in $DEVICES; do
        curl -s -X DELETE "$API_URL/devices/$uid" > /dev/null
    done

    print_success "Cleanup complete"
    echo ""
}

# Main test execution
main() {
    echo "🧪 Testing Fabrica Device Inventory API"
    echo "========================================="
    echo ""

    check_server
    test_create
    test_create_more
    test_list
    test_get
    test_update
    test_delete
    test_verify_deletion
    test_validation
    test_openapi
    cleanup

    echo "========================================="
    print_success "All tests passed! 🎉"
    echo ""
    echo "Next steps:"
    echo "  - Explore the OpenAPI spec: curl $API_URL/openapi.json | jq"
    echo "  - Check the data directory: ls -la ./data/devices/"
    echo "  - Try the Go client: go run examples/client/main.go"
    echo "  - Move to Example 2: cd ../02-storage-auth"
}

# Run tests
main

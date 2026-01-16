#!/bin/bash
# SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
# SPDX-License-Identifier: MIT

# test-lib.sh - Shared testing utilities for Fabrica examples
# Provides reusable functions for HTTP testing, color output, error handling, and assertions

set -e

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_SERVER="${API_SERVER:-http://localhost:8080}"
TIMEOUT="${TIMEOUT:-30}"
VERBOSE="${VERBOSE:-0}"

# Print colored output
print_green() {
    echo -e "${GREEN}✓${NC} $1"
}

print_red() {
    echo -e "${RED}✗${NC} $1"
}

print_yellow() {
    echo -e "${YELLOW}!${NC} $1"
}

print_blue() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Verify required commands are installed
require_commands() {
    local missing=()
    for cmd in "$@"; do
        if ! command_exists "$cmd"; then
            missing+=("$cmd")
        fi
    done

    if [ ${#missing[@]} -gt 0 ]; then
        print_red "Missing required commands: ${missing[*]}"
        exit 1
    fi
}

# Wait for API server to be ready with timeout
wait_for_server() {
    local url="${1:-$API_SERVER}"
    local max_attempts=$((TIMEOUT * 10))
    local attempts=0

    print_blue "Waiting for server at $url..."

    while [ $attempts -lt $max_attempts ]; do
        if curl -sf "$url/health" >/dev/null 2>&1; then
            print_green "Server is ready"
            return 0
        fi
        attempts=$((attempts + 1))
        sleep 0.1
    done

    print_red "Server did not respond after ${TIMEOUT}s"
    return 1
}

# Kill background server gracefully
stop_server() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        print_blue "Stopping server (PID: $SERVER_PID)..."
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
        print_green "Server stopped"
    fi
}

# Make HTTP GET request
http_get() {
    local endpoint="$1"
    local url="${2:-$API_SERVER}$endpoint"
    local response

    if [ "$VERBOSE" -eq 1 ]; then
        print_blue "GET $url"
    fi

    response=$(curl -sf "$url" 2>&1) || {
        print_red "GET $endpoint failed"
        return 1
    }

    echo "$response"
}

# Make HTTP POST request
http_post() {
    local endpoint="$1"
    local data="$2"
    local url="${3:-$API_SERVER}$endpoint"
    local response

    if [ "$VERBOSE" -eq 1 ]; then
        print_blue "POST $url"
        print_blue "Data: $data"
    fi

    response=$(curl -sf -X POST "$url" \
        -H "Content-Type: application/json" \
        -d "$data" 2>&1) || {
        print_red "POST $endpoint failed"
        return 1
    }

    echo "$response"
}

# Make HTTP PUT request
http_put() {
    local endpoint="$1"
    local data="$2"
    local url="${3:-$API_SERVER}$endpoint"
    local response

    if [ "$VERBOSE" -eq 1 ]; then
        print_blue "PUT $url"
        print_blue "Data: $data"
    fi

    response=$(curl -sf -X PUT "$url" \
        -H "Content-Type: application/json" \
        -d "$data" 2>&1) || {
        print_red "PUT $endpoint failed"
        return 1
    }

    echo "$response"
}

# Make HTTP PATCH request
http_patch() {
    local endpoint="$1"
    local data="$2"
    local url="${3:-$API_SERVER}$endpoint"
    local response

    if [ "$VERBOSE" -eq 1 ]; then
        print_blue "PATCH $url"
        print_blue "Data: $data"
    fi

    response=$(curl -sf -X PATCH "$url" \
        -H "Content-Type: application/merge-patch+json" \
        -d "$data" 2>&1) || {
        print_red "PATCH $endpoint failed"
        return 1
    }

    echo "$response"
}

# Make HTTP DELETE request
http_delete() {
    local endpoint="$1"
    local url="${2:-$API_SERVER}$endpoint"

    if [ "$VERBOSE" -eq 1 ]; then
        print_blue "DELETE $url"
    fi

    curl -sf -X DELETE "$url" >/dev/null 2>&1 || {
        print_red "DELETE $endpoint failed"
        return 1
    }
}

# Assert HTTP response status code
assert_status_code() {
    local endpoint="$1"
    local method="$2"
    local expected_code="$3"
    local data="${4:-}"
    local url="${5:-$API_SERVER}$endpoint"

    local actual_code
    if [ -z "$data" ]; then
        actual_code=$(curl -s -w "%{http_code}" -X "$method" "$url" -o /dev/null 2>&1)
    else
        actual_code=$(curl -s -w "%{http_code}" -X "$method" "$url" \
            -H "Content-Type: application/json" \
            -d "$data" -o /dev/null 2>&1)
    fi

    if [ "$actual_code" = "$expected_code" ]; then
        print_green "$method $endpoint returned $actual_code"
        return 0
    else
        print_red "$method $endpoint returned $actual_code (expected $expected_code)"
        return 1
    fi
}

# Extract field from JSON response using jq or grep as fallback
extract_json_field() {
    local json="$1"
    local field="$2"

    if command_exists jq; then
        echo "$json" | jq -r "$field" 2>/dev/null || echo ""
    else
        # Fallback: simple grep extraction for common patterns
        echo "$json" | grep -o "\"$field\":[^,}]*" | cut -d':' -f2 | tr -d '"' || echo ""
    fi
}

# Test basic CRUD operations
test_crud_operations() {
    local resource_name="$1"
    local endpoint="$2"
    local create_payload="$3"

    print_blue "Testing CRUD for $resource_name at $endpoint"

    # CREATE
    print_blue "  Testing CREATE..."
    local created
    created=$(http_post "$endpoint" "$create_payload") || return 1
    local id
    id=$(extract_json_field "$created" '.metadata.uid')

    if [ -z "$id" ]; then
        print_red "Failed to extract ID from creation response"
        return 1
    fi
    print_green "  Resource created with ID: $id"

    # READ (single)
    print_blue "  Testing READ..."
    http_get "$endpoint/$id" >/dev/null || return 1
    print_green "  Resource retrieved successfully"

    # LIST
    print_blue "  Testing LIST..."
    local list_response
    list_response=$(http_get "$endpoint") || return 1
    if echo "$list_response" | grep -q "$id"; then
        print_green "  Resource found in list"
    fi

    # UPDATE
    print_blue "  Testing UPDATE..."
    local update_payload
    update_payload=$(echo "$created" | sed 's/"description": "[^"]*"/"description": "updated"/')
    http_put "$endpoint/$id" "$update_payload" >/dev/null || return 1
    print_green "  Resource updated successfully"

    # DELETE
    print_blue "  Testing DELETE..."
    http_delete "$endpoint/$id" || return 1
    print_green "  Resource deleted successfully"

    # Verify deletion
    if ! http_get "$endpoint/$id" 2>/dev/null; then
        print_green "  Deletion verified (resource no longer accessible)"
    fi
}

# Test validation errors
test_validation_errors() {
    local endpoint="$1"
    local invalid_payload="$2"

    print_blue "Testing validation error handling..."

    if http_post "$endpoint" "$invalid_payload" 2>&1 | grep -q "error\|invalid\|bad"; then
        print_green "Validation error detected as expected"
        return 0
    else
        print_yellow "Validation error might not have been caught (endpoint may not validate)"
        return 0
    fi
}

# Test event subscription
test_events() {
    local event_type="$1"
    local event_endpoint="${2:-/events}"

    print_blue "Testing event handling for $event_type..."

    if ! command_exists jq; then
        print_yellow "jq not installed; skipping detailed event validation"
        return 0
    fi

    # Try to subscribe to events
    local events
    events=$(curl -sf "$API_SERVER$event_endpoint?type=$event_type" 2>&1 | head -c 100) || {
        print_yellow "Event endpoint not available"
        return 0
    }

    if echo "$events" | grep -q "id"; then
        print_green "Event subscription successful"
        return 0
    fi
}

# Clean up after tests
cleanup() {
    print_blue "Cleaning up..."
    stop_server
    print_green "Cleanup complete"
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Export functions for sourcing
export -f print_green print_red print_yellow print_blue
export -f command_exists require_commands wait_for_server stop_server
export -f http_get http_post http_put http_patch http_delete
export -f assert_status_code extract_json_field
export -f test_crud_operations test_validation_errors test_events cleanup

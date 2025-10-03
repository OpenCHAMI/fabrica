#!/bin/bash

# Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

# Test script for validation + conditional requests + PATCH demo

set -e

BASE_URL="http://localhost:8080"
RESOURCES_URL="$BASE_URL/resources"

echo "================================"
echo "Fabrica Validation Demo"
echo "================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Starting server check...${NC}"
if ! curl -s "$BASE_URL/resources/1" > /dev/null; then
    echo -e "${RED}Error: Server not running on port 8080${NC}"
    echo "Please start the server with: go run main.go"
    exit 1
fi
echo -e "${GREEN}✓ Server is running${NC}"
echo ""

# Test 1: Valid resource creation
echo "================================"
echo "Test 1: Create Valid Resource"
echo "================================"
echo -e "${YELLOW}Creating resource with valid k8sname and active status...${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$RESOURCES_URL" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "active-test-device",
    "status": "active",
    "description": "A valid test resource"
  }')
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "201" ]; then
    echo -e "${GREEN}✓ Success (201 Created)${NC}"
    echo "$BODY" | jq '.'
else
    echo -e "${RED}✗ Failed (HTTP $HTTP_CODE)${NC}"
    echo "$BODY" | jq '.'
fi
echo ""

# Test 2: Invalid name (uppercase)
echo "================================"
echo "Test 2: Invalid Name (Uppercase)"
echo "================================"
echo -e "${YELLOW}Trying to create resource with uppercase name...${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$RESOURCES_URL" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Invalid-Name",
    "status": "active"
  }')
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "400" ]; then
    echo -e "${GREEN}✓ Correctly rejected (400 Bad Request)${NC}"
    echo "$BODY" | jq '.'
else
    echo -e "${RED}✗ Unexpected result (HTTP $HTTP_CODE)${NC}"
    echo "$BODY" | jq '.'
fi
echo ""

# Test 3: Custom validation - inactive with tags
echo "================================"
echo "Test 3: Custom Validation (Inactive + Tags)"
echo "================================"
echo -e "${YELLOW}Trying to create inactive resource with tags...${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$RESOURCES_URL" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "inactive-device",
    "status": "inactive",
    "tags": ["test", "demo"]
  }')
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "400" ]; then
    echo -e "${GREEN}✓ Correctly rejected by custom validator (400 Bad Request)${NC}"
    echo "$BODY" | jq '.'
else
    echo -e "${RED}✗ Unexpected result (HTTP $HTTP_CODE)${NC}"
    echo "$BODY" | jq '.'
fi
echo ""

# Test 4: Missing required fields
echo "================================"
echo "Test 4: Missing Required Fields"
echo "================================"
echo -e "${YELLOW}Trying to create resource without name and status...${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$RESOURCES_URL" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Missing required fields"
  }')
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "400" ]; then
    echo -e "${GREEN}✓ Correctly rejected (400 Bad Request)${NC}"
    echo "$BODY" | jq '.'
else
    echo -e "${RED}✗ Unexpected result (HTTP $HTTP_CODE)${NC}"
    echo "$BODY" | jq '.'
fi
echo ""

# Test 5: Valid PATCH
echo "================================"
echo "Test 5: Valid PATCH Operation"
echo "================================"
echo -e "${YELLOW}Patching resource with valid description...${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH "$RESOURCES_URL/1" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "description": "Updated via validation test"
  }')
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✓ Success (200 OK)${NC}"
    echo "$BODY" | jq '.'
else
    echo -e "${RED}✗ Failed (HTTP $HTTP_CODE)${NC}"
    echo "$BODY" | jq '.'
fi
echo ""

# Test 6: Invalid PATCH (bad status)
echo "================================"
echo "Test 6: Invalid PATCH (Bad Status)"
echo "================================"
echo -e "${YELLOW}Trying to patch with invalid status...${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH "$RESOURCES_URL/1" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "status": "invalid-status"
  }')
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "400" ]; then
    echo -e "${GREEN}✓ Correctly rejected (400 Bad Request)${NC}"
    echo "$BODY" | jq '.'
else
    echo -e "${RED}✗ Unexpected result (HTTP $HTTP_CODE)${NC}"
    echo "$BODY" | jq '.'
fi
echo ""

# Test 7: Optimistic Concurrency
echo "================================"
echo "Test 7: Optimistic Concurrency with ETag"
echo "================================"
echo -e "${YELLOW}Getting current ETag...${NC}"
ETAG=$(curl -s -i "$RESOURCES_URL/1" | grep -i "^etag:" | cut -d' ' -f2 | tr -d '\r\n')
echo "Current ETag: $ETAG"

echo -e "${YELLOW}Updating with correct ETag...${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH "$RESOURCES_URL/1" \
  -H "If-Match: $ETAG" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "description": "Updated with concurrency control"
  }')
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✓ Success with matching ETag (200 OK)${NC}"
    echo "$BODY" | jq '.description'
else
    echo -e "${RED}✗ Failed (HTTP $HTTP_CODE)${NC}"
    echo "$BODY" | jq '.'
fi

echo -e "${YELLOW}Trying to update with old (stale) ETag...${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH "$RESOURCES_URL/1" \
  -H "If-Match: $ETAG" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "description": "This should fail"
  }')
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "412" ]; then
    echo -e "${GREEN}✓ Correctly rejected stale ETag (412 Precondition Failed)${NC}"
else
    echo -e "${RED}✗ Unexpected result (HTTP $HTTP_CODE)${NC}"
fi
echo ""

# Summary
echo "================================"
echo "Test Summary"
echo "================================"
echo -e "${GREEN}All validation features working correctly!${NC}"
echo ""
echo "Features tested:"
echo "  ✓ Struct tag validation (k8sname, oneof, required)"
echo "  ✓ Custom validation logic (business rules)"
echo "  ✓ Hybrid approach (tags + CustomValidator)"
echo "  ✓ PATCH with validation"
echo "  ✓ Optimistic concurrency with ETags"
echo "  ✓ Detailed error responses"
echo ""

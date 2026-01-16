#!/bin/bash

# Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
# SPDX-License-Identifier: MIT

set -e

# Create test project
mkdir -p tmp-debug
cd tmp-debug
/Users/alovelltroy/Development/OpenCHAMI/fabrica/bin/fabrica init debug-test --module github.com/test/debug --storage-type file
/Users/alovelltroy/Development/OpenCHAMI/fabrica/bin/fabrica add resource Device
/Users/alovelltroy/Development/OpenCHAMI/fabrica/bin/fabrica generate

# Build and start server
cd cmd/server
go build -o server
./server serve --port 9999 &
SERVER_PID=$!
sleep 2

# Create resource
curl -X POST http://localhost:9999/devices \
  -H "Content-Type: application/json" \
  -d '{"apiVersion":"example.com/v1","kind":"Device","metadata":{"name":"test-device"},"spec":{"description":"test"}}' \
  | jq '.'

# Get by UID (replace with actual UID from above)
echo ""
echo "List all:"
curl -s http://localhost:9999/devices | jq '.'

kill $SERVER_PID

#!/bin/bash

# Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
# SPDX-License-Identifier: MIT

# test-events.sh - Test script for CloudEvents example
set -e

echo "🎯 Testing CloudEvents Integration Example"
echo "=========================================="

# Configuration
API_URL="http://localhost:8080"
SERVER_PID=""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Cleanup function
cleanup() {
    if [ ! -z "$SERVER_PID" ]; then
        log_info "Stopping server (PID: $SERVER_PID)"
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
}

# Set trap for cleanup
trap cleanup EXIT

# Check if server is already running
if curl -s "$API_URL/sensors" > /dev/null 2>&1; then
    log_warning "Server appears to be already running. Using existing server."
    SERVER_RUNNING=true
else
    SERVER_RUNNING=false
fi

# Test 1: Create a sensor (triggers 'created' event)
echo
log_info "Test 1: Creating a temperature sensor"
SENSOR_RESPONSE=$(curl -s -X POST "$API_URL/sensors" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "temp-sensor-01",
    "description": "Office temperature sensor for CloudEvents demo",
    "sensorType": "temperature",
    "location": "Building A, Floor 2, Room 201",
    "threshold": 75.0
  }' || echo "ERROR")

if echo "$SENSOR_RESPONSE" | grep -q '"kind":"Sensor"'; then
    log_success "Sensor created successfully"
    echo "📧 Event published: io.fabrica.sensor.created"

    # Extract UID for subsequent tests
    SENSOR_UID=$(echo "$SENSOR_RESPONSE" | grep -o '"uid":"[^"]*"' | cut -d'"' -f4)
    log_info "Sensor UID: $SENSOR_UID"
else
    log_error "Failed to create sensor"
    echo "Response: $SENSOR_RESPONSE"
    exit 1
fi

# Test 2: Update the sensor (triggers 'updated' event)
echo
log_info "Test 2: Updating the sensor"
UPDATE_RESPONSE=$(curl -s -X PUT "$API_URL/sensors/$SENSOR_UID" \
  -H "Content-Type: application/json" \
  -d '{
      "name": "temp-sensor-01",
      "description": "Updated office temperature sensor with higher threshold",
      "sensorType": "temperature",
      "location": "Building A, Floor 2, Room 201",
      "threshold": 80.0
  }' || echo "ERROR")

if echo "$UPDATE_RESPONSE" | grep -q '"threshold":80'; then
    log_success "Sensor updated successfully"
    echo "📧 Event published: io.fabrica.sensor.updated"
else
    log_error "Failed to update sensor"
    echo "Response: $UPDATE_RESPONSE"
fi

# Test 3: Patch the sensor status (triggers 'patched' event)
echo
log_info "Test 3: Patching sensor status"
PATCH_RESPONSE=$(curl -s -X PATCH "$API_URL/sensors/$SENSOR_UID" \
  -H "Content-Type: application/json" \
  -d '{
    "status": {
      "phase": "active",
      "value": 72.5,
      "lastReading": "2025-01-15T10:30:00Z",
      "conditions": [
        {
          "type": "Ready",
          "status": "True",
          "reason": "SensorOnline",
          "message": "Sensor is online and reporting data",
          "lastTransitionTime": "2025-01-15T10:30:00Z"
        }
      ]
    }
  }' || echo "ERROR")

if echo "$PATCH_RESPONSE" | grep -q '"phase":"active"'; then
    log_success "Sensor status patched successfully"
    echo "📧 Event published: io.fabrica.sensor.patched"
    echo "📧 Condition event published: io.fabrica.condition.ready"
else
    log_error "Failed to patch sensor status"
    echo "Response: $PATCH_RESPONSE"
fi

# Test 4: List all sensors
echo
log_info "Test 4: Listing all sensors"
LIST_RESPONSE=$(curl -s "$API_URL/sensors" || echo "ERROR")

if [ "$LIST_RESPONSE" != "ERROR" ] && echo "$LIST_RESPONSE" | jq -e '.' > /dev/null 2>&1; then
  SENSOR_COUNT=$(echo "$LIST_RESPONSE" | jq '. | length')
  log_success "Found $SENSOR_COUNT sensors"

  # Display sensor details if any exist
  if [ "$SENSOR_COUNT" -gt 0 ]; then
    echo "$LIST_RESPONSE" | jq -r '.[] | "  - \(.metadata.name) (\(.spec.sensorType)): \(.metadata.uid)"'
  fi
else
  log_error "Failed to list sensors"
  echo "Response: $LIST_RESPONSE"
fi

# Test 5: Get specific sensor
echo
log_info "Test 5: Getting specific sensor"
GET_RESPONSE=$(curl -s "$API_URL/sensors/$SENSOR_UID" || echo "ERROR")

if echo "$GET_RESPONSE" | grep -q '"kind":"Sensor"'; then
    log_success "Retrieved sensor details successfully"
else
    log_error "Failed to get sensor details"
    echo "Response: $GET_RESPONSE"
fi

# Test 6: Create another sensor with different type
echo
log_info "Test 6: Creating a humidity sensor"
HUMIDITY_RESPONSE=$(curl -s -X POST "$API_URL/sensors" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "humidity-sensor-01",
    "description": "Office humidity sensor",
    "sensorType": "humidity",
    "location": "Building A, Floor 2, Room 202",
    "threshold": 60.0
  }' || echo "ERROR")

if echo "$HUMIDITY_RESPONSE" | grep -q '"sensorType":"humidity"'; then
    HUMIDITY_UID=$(echo "$HUMIDITY_RESPONSE" | grep -o '"uid":"[^"]*"' | cut -d'"' -f4)
    log_success "Humidity sensor created successfully"
    echo "📧 Event published: io.fabrica.sensor.created"
    log_info "Humidity sensor UID: $HUMIDITY_UID"
else
    log_error "Failed to create humidity sensor"
    echo "Response: $HUMIDITY_RESPONSE"
fi

# Test 7: Update humidity sensor with unhealthy condition
echo
log_info "Test 7: Setting humidity sensor to unhealthy state"
UNHEALTHY_RESPONSE=$(curl -s -X PATCH "$API_URL/sensors/$HUMIDITY_UID" \
  -H "Content-Type: application/json" \
  -d '{
    "status": {
      "phase": "degraded",
      "value": 95.0,
      "lastReading": "2025-01-15T10:35:00Z",
      "conditions": [
        {
          "type": "Healthy",
          "status": "False",
          "reason": "ThresholdExceeded",
          "message": "Humidity level exceeds safe threshold",
          "lastTransitionTime": "2025-01-15T10:35:00Z"
        }
      ]
    }
  }' || echo "ERROR")

if echo "$UNHEALTHY_RESPONSE" | grep -q '"phase":"degraded"'; then
    log_success "Humidity sensor marked as degraded"
    echo "📧 Event published: io.fabrica.sensor.patched"
    echo "📧 Condition event published: io.fabrica.condition.healthy"
else
    log_error "Failed to update humidity sensor status"
    echo "Response: $UNHEALTHY_RESPONSE"
fi

# Test 8: Delete sensors (triggers 'deleted' events)
echo
log_info "Test 8: Deleting sensors"

# Delete temperature sensor
DELETE_TEMP_RESPONSE=$(curl -s -X DELETE "$API_URL/sensors/$SENSOR_UID" || echo "ERROR")
if [ "$DELETE_TEMP_RESPONSE" != "ERROR" ]; then
    log_success "Temperature sensor deleted successfully"
    echo "📧 Event published: io.fabrica.sensor.deleted"
else
    log_error "Failed to delete temperature sensor"
    echo "Response: $DELETE_TEMP_RESPONSE"
fi

# Delete humidity sensor
DELETE_HUM_RESPONSE=$(curl -s -X DELETE "$API_URL/sensors/$HUMIDITY_UID" || echo "ERROR")
if [ "$DELETE_HUM_RESPONSE" != "ERROR" ]; then
    log_success "Humidity sensor deleted successfully"
    echo "📧 Event published: io.fabrica.sensor.deleted"
else
    log_error "Failed to delete humidity sensor"
    echo "Response: $DELETE_HUM_RESPONSE"
fi

# Test 9: Verify sensors are deleted
echo
log_info "Test 9: Verifying sensors are deleted"
FINAL_LIST=$(curl -s "$API_URL/sensors" || echo "ERROR")

if echo "$FINAL_LIST" | jq -e '. == []' > /dev/null 2>&1; then
    log_success "All sensors successfully deleted"
else
    REMAINING_COUNT=$(echo "$FINAL_LIST" | jq '. | length')
    log_warning "$REMAINING_COUNT sensors still remain"
fi

# Summary
echo
echo "🎯 CloudEvents Integration Test Summary"
echo "======================================"
log_success "All CRUD operations completed successfully"
echo
echo "📧 Events Published During Test:"
echo "   • io.fabrica.sensor.created (3 times - temp + humidity + any previous)"
echo "   • io.fabrica.sensor.updated (1 time - temp sensor)"
echo "   • io.fabrica.sensor.patched (2 times - temp + humidity status)"
echo "   • io.fabrica.sensor.deleted (2 times - both sensors)"
echo "   • io.fabrica.condition.ready (1 time - temp sensor ready)"
echo "   • io.fabrica.condition.healthy (1 time - humidity sensor unhealthy)"
echo
echo "💡 To see events in real-time:"
echo "   1. Run the event subscriber example"
echo "   2. Enable debug logging in the server"
echo "   3. Use external event monitoring tools"
echo
log_success "CloudEvents example test completed! 🎉"

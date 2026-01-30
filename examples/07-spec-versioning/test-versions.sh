#!/usr/bin/env bash

# Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

set -euo pipefail

# API base URL (override with API_URL env)
API_URL="${API_URL:-http://localhost:8080}"

BLUE='\033[0;34m'; GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
step(){ echo -e "${BLUE}▶ $*${NC}"; }
ok(){ echo -e "${GREEN}✅ $*${NC}"; }
warn(){ echo -e "${YELLOW}⚠️  $*${NC}"; }
fail(){ echo -e "${RED}❌ $*${NC}"; exit 1; }

require_server(){
  step "Checking server at $API_URL/health"
  curl -sf "$API_URL/health" >/dev/null || fail "Server is not running at $API_URL"
  ok "Server is running"
}

create_sensor(){
  step "Create Sensor"
  RESP=$(curl -s -i -H 'Content-Type: application/json' -d '{"name":"ex1","description":"first"}' "$API_URL/sensors")
  BODY=$(echo "$RESP" | tr -d '\r' | awk 'f{print} /^$/{f=1}')
  UID=$(echo "$BODY" | grep -o '"uid":"[^"]*"' | head -1 | cut -d '"' -f4)
  VER=$(echo "$BODY" | grep -o '"version":"[^"]*"' | head -1 | cut -d '"' -f4)
  [[ -n "$UID" ]] || fail "Failed to parse UID from create response"
  [[ -n "$VER" ]] || fail "status.version missing in create response"
  ok "Created Sensor UID=$UID version=$VER"
  echo "$UID"
}

update_spec(){
  local uid="$1"; step "Update Spec (PUT)"
  PRE=$(curl -s "$API_URL/sensors/$uid" | tr -d '\r' | grep -o '"version":"[^"]*"' | head -1 | cut -d '"' -f4)
  RESP=$(curl -s -i -X PUT -H 'Content-Type: application/json' -d '{"description":"second"}' "$API_URL/sensors/$uid")
  POST=$(echo "$RESP" | tr -d '\r' | awk 'f{print} /^$/{f=1}' | grep -o '"version":"[^"]*"' | head -1 | cut -d '"' -f4)
  [[ "$PRE" != "$POST" ]] || fail "Expected version change on PUT"
  ok "Version advanced: $PRE -> $POST"
}

patch_spec(){
  local uid="$1"; step "Patch Spec (PATCH)"
  PRE=$(curl -s "$API_URL/sensors/$uid" | tr -d '\r' | grep -o '"version":"[^"]*"' | head -1 | cut -d '"' -f4)
  RESP=$(curl -s -i -X PATCH -H 'Content-Type: application/merge-patch+json' -d '{"description":"third"}' "$API_URL/sensors/$uid")
  POST=$(echo "$RESP" | tr -d '\r' | awk 'f{print} /^$/{f=1}' | grep -o '"version":"[^"]*"' | head -1 | cut -d '"' -f4)
  [[ "$PRE" != "$POST" ]] || fail "Expected version change on PATCH"
  ok "Version advanced: $PRE -> $POST"
}

status_update_no_change(){
  local uid="$1"; step "Status Update (no version change)"
  PRE=$(curl -s "$API_URL/sensors/$uid" | tr -d '\r' | grep -o '"version":"[^"]*"' | head -1 | cut -d '"' -f4)
  curl -s -i -X PUT -H 'Content-Type: application/json' -d '{"ready":true,"phase":"Active"}' "$API_URL/sensors/$uid/status" >/dev/null
  POST=$(curl -s "$API_URL/sensors/$uid" | tr -d '\r' | grep -o '"version":"[^"]*"' | head -1 | cut -d '"' -f4)
  [[ "$PRE" == "$POST" ]] || fail "status.version should not change on status update"
  ok "status.version preserved: $PRE"
}

list_and_get_versions(){
  local uid="$1"; step "List and fetch versions"
  LIST=$(curl -s "$API_URL/sensors/$uid/versions")
  COUNT=$(echo "$LIST" | grep -o '"versionId":"' | wc -l | tr -d ' ')
  [[ "$COUNT" -ge 2 ]] || warn "Only $COUNT versions found (expected >=2)"
  VID=$(echo "$LIST" | grep -o '"versionId":"[^"]*"' | head -1 | cut -d '"' -f4)
  [[ -n "$VID" ]] || fail "Failed to parse versionId from list"
  curl -s "$API_URL/sensors/$uid/versions/$VID" >/dev/null || fail "Failed to GET version $VID"
  ok "Listed $COUNT versions; fetched $VID"
}

main(){
  echo -e "\n\nTesting Spec Version History at $API_URL\n"
  require_server
  UID=$(create_sensor)
  update_spec "$UID"
  patch_spec "$UID"
  status_update_no_change "$UID"
  list_and_get_versions "$UID"
  echo -e "\nAll checks passed."
}

main "$@"

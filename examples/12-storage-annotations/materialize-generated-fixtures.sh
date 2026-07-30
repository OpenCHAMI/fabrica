#!/usr/bin/env bash
# Copyright © 2026 OpenCHAMI Contributors
# SPDX-License-Identifier: MIT

set -euo pipefail

project_dir="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
mkdir -p "${project_dir}/cmd/server" "${project_dir}/cmd/verify-storage"
cp "${project_dir}/testdata/server-main.go.txt" "${project_dir}/cmd/server/main.go"
cp "${project_dir}/testdata/server-storage.go.txt" "${project_dir}/cmd/server/storage.go"
cp "${project_dir}/testdata/verify-storage-main.go.txt" "${project_dir}/cmd/verify-storage/main.go"

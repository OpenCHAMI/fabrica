// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package constants contains constant values for tokensmith code generation.
package constants

// TokenSmithModulePath is the Go module path for TokenSmith.
const TokenSmithModulePath = "github.com/OpenCHAMI/tokensmith"

// TokenSmithVersion pins the TokenSmith version used by generated services.
//
// Keep this deterministic to ensure stable regeneration results.
// Until a stable release is available, this tracks the current pseudo-version from main.
const TokenSmithVersion = "v0.0.0-20260324140532-0985357a5e2b"

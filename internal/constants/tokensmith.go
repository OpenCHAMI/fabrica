// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package constants contains constant values for tokensmith code generation.
package constants

// TokenSmithModulePath is the Go module path for TokenSmith.
const TokenSmithModulePath = "github.com/openchami/tokensmith"

// TokenSmithAuthNPackagePath is the import path for the recommended AuthN middleware.
const TokenSmithAuthNPackagePath = TokenSmithModulePath + "/pkg/authn"

// TokenSmithVersion pins the TokenSmith version used by generated services.
//
// Keep this deterministic to ensure stable regeneration results.
const TokenSmithVersion = "v0.3.0"

// TokenSmithGoVersion is the minimum Go version required by the pinned TokenSmith release.
const TokenSmithGoVersion = "1.24.0"

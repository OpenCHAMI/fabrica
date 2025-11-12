// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package fabrica provides common types for versioned API definitions.
package fabrica

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// Metadata is an alias for resource.Metadata, providing a cleaner import path
// for versioned API types.
//
// This allows versioned resources to use fabrica.Metadata instead of
// resource.Metadata, which reads more naturally in API definitions.
type Metadata = resource.Metadata

// Condition is an alias for resource.Condition, providing a cleaner import path
// for versioned API types.
//
// This allows versioned resources to use fabrica.Condition instead of
// resource.Condition, which reads more naturally in API definitions.
type Condition = resource.Condition

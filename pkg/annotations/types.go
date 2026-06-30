// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package annotations provides parsing and validation for Fabrica annotations.
//
// Fabrica uses Go comment annotations (similar to Kubernetes code generators) to
// configure code generation behavior. Annotations are written as comments above
// types and fields using the +fabrica: prefix.
//
// Example:
//
//	// +fabrica:resource
//	// +fabrica:storage=dedicated
//	type Token struct {
//	    Spec TokenSpec
//	}
//
//	type TokenSpec struct {
//	    // +fabrica:field:storage=hashed:bcrypt:cost=12
//	    // +fabrica:field:sensitive
//	    Value string `json:"value"`
//	}
package annotations

import (
	"fmt"
	"strconv"
	"strings"
)

// StorageMode defines how a resource is persisted
type StorageMode string

const (
	// StorageModeGeneric stores resources in a single generic table with JSON spec (default)
	StorageModeGeneric StorageMode = "generic"
	// StorageModeDedicated generates a dedicated table per resource with flattened fields
	StorageModeDedicated StorageMode = "dedicated"
)

// IndexType defines the type of database index
type IndexType string

const (
	// IndexTypeBTree is a standard B-tree index (default)
	IndexTypeBTree IndexType = "btree"
	// IndexTypeGIN is a PostgreSQL GIN index for full-text and JSON search
	IndexTypeGIN IndexType = "gin"
	// IndexTypeGiST is a PostgreSQL GiST index for spatial data
	IndexTypeGiST IndexType = "gist"
	// IndexTypeHash is a hash index
	IndexTypeHash IndexType = "hash"
)

// StorageType defines how a field value is stored
type StorageType string

const (
	// StorageTypeDefault stores the field as-is (default)
	StorageTypeDefault StorageType = "default"
	// StorageTypeHashed stores a hash of the field value
	StorageTypeHashed StorageType = "hashed"
	// StorageTypeEncrypted stores an encrypted value
	StorageTypeEncrypted StorageType = "encrypted"
)

// HashAlgorithm defines the hashing algorithm
type HashAlgorithm string

const (
	// HashAlgorithmBcrypt uses bcrypt hashing
	HashAlgorithmBcrypt HashAlgorithm = "bcrypt"
	// HashAlgorithmArgon2 uses argon2 hashing
	HashAlgorithmArgon2 HashAlgorithm = "argon2"
	// HashAlgorithmSHA256 uses SHA-256 hashing
	HashAlgorithmSHA256 HashAlgorithm = "sha256"
)

// ResourceAnnotations contains all annotations for a resource type
type ResourceAnnotations struct {
	// IsResource indicates this is a Fabrica resource (+fabrica:resource)
	IsResource bool

	// StorageMode defines how the resource is persisted
	StorageMode StorageMode

	// Fields maps field names to their annotations
	Fields map[string]*FieldAnnotations

	// RawAnnotations contains unparsed annotation lines for debugging
	RawAnnotations []string
}

// NewResourceAnnotations creates a new ResourceAnnotations with defaults
func NewResourceAnnotations() *ResourceAnnotations {
	return &ResourceAnnotations{
		IsResource:  false,
		StorageMode: StorageModeGeneric,
		Fields:      make(map[string]*FieldAnnotations),
	}
}

// FieldAnnotations contains all annotations for a single field
type FieldAnnotations struct {
	// FieldName is the Go field name
	FieldName string

	// Storage configuration
	Storage *StorageConfig

	// Sensitive marks field as sensitive (exclude from logs)
	Sensitive bool

	// Immutable prevents field updates after creation
	Immutable bool

	// Index configuration
	Index *IndexConfig

	// Default value (database-level default)
	Default string

	// Unique constraint
	Unique bool

	// RawAnnotations contains unparsed annotation lines for debugging
	RawAnnotations []string
}

// NewFieldAnnotations creates a new FieldAnnotations with defaults
func NewFieldAnnotations(fieldName string) *FieldAnnotations {
	return &FieldAnnotations{
		FieldName: fieldName,
	}
}

// StorageConfig defines how a field value is stored in the database
type StorageConfig struct {
	// Type defines the storage strategy
	Type StorageType

	// Hash configuration (when Type == StorageTypeHashed)
	Hash *HashConfig

	// Encryption configuration (when Type == StorageTypeEncrypted)
	Encryption *EncryptionConfig
}

// HashConfig defines hashing parameters
type HashConfig struct {
	// Algorithm is the hash algorithm to use
	Algorithm HashAlgorithm

	// Cost is the computational cost parameter (algorithm-specific)
	// For bcrypt: 4-31 (default 12)
	// For argon2: memory cost in KB
	Cost int
}

// EncryptionConfig defines encryption parameters
type EncryptionConfig struct {
	// Algorithm is the encryption algorithm
	Algorithm string // e.g., "aes256", "aes128"

	// KeySource defines where encryption keys are stored
	KeySource string // e.g., "env", "vault", "kms"
}

// IndexConfig defines database index parameters
type IndexConfig struct {
	// Type is the index type
	Type IndexType

	// Unique indicates a unique index
	Unique bool

	// Name is an optional custom index name
	Name string
}

// ParseError represents an annotation parsing error
type ParseError struct {
	Line    string // The annotation line that failed to parse
	Message string // Error message
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse annotation %q: %s", e.Line, e.Message)
}

// ValidationError represents an annotation validation error
type ValidationError struct {
	Annotation string // The annotation that failed validation
	Message    string // Error message
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid annotation %q: %s", e.Annotation, e.Message)
}

// ParseAnnotationValue extracts key-value pairs from annotation string
//
// Supports formats:
//   - Simple: "+fabrica:storage=dedicated"
//   - Nested: "+fabrica:field:storage=hashed:bcrypt:cost=12"
//
// Returns a slice of parts: ["storage=dedicated"] or ["field", "storage=hashed", "bcrypt", "cost=12"]
func ParseAnnotationValue(annotation string) []string {
	// Trim leading/trailing whitespace first
	annotation = strings.TrimSpace(annotation)

	// Remove +fabrica: prefix
	annotation = strings.TrimPrefix(annotation, "+fabrica:")
	annotation = strings.TrimSpace(annotation)

	// Split on colons
	parts := strings.Split(annotation, ":")

	// Trim whitespace from each part and filter empty parts
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// ParseKeyValue splits a "key=value" string
func ParseKeyValue(part string) (key, value string, hasValue bool) {
	if idx := strings.Index(part, "="); idx >= 0 {
		return part[:idx], part[idx+1:], true
	}
	return part, "", false
}

// ParseIntValue parses an integer from a string, with validation
func ParseIntValue(value string, minVal, maxVal int) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer: %s", value)
	}
	if n < minVal || n > maxVal {
		return 0, fmt.Errorf("value %d out of range [%d, %d]", n, minVal, maxVal)
	}
	return n, nil
}

// IsFabricaAnnotation checks if a comment line is a Fabrica annotation
func IsFabricaAnnotation(line string) bool {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "//")
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "+fabrica:")
}

// CleanAnnotationLine cleans a comment line to extract the annotation
func CleanAnnotationLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "//")
	line = strings.TrimSpace(line)
	return line
}

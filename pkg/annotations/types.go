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

// MigrationPolicy defines how far a schema migration may go when altering a
// resource's table.
type MigrationPolicy string

const (
	// MigrationPolicyUnrestricted allows any migration (default)
	MigrationPolicyUnrestricted MigrationPolicy = "unrestricted"
	// MigrationPolicyAdditiveOnly permits only additive changes (new columns,
	// new indexes); drops and narrowing type changes are rejected
	MigrationPolicyAdditiveOnly MigrationPolicy = "additive-only"
)

// RelationKind defines the cardinality of a relation between resources
type RelationKind string

const (
	// RelationBelongsTo is a many-to-one edge to the target resource
	RelationBelongsTo RelationKind = "belongs-to"
	// RelationHasMany is a one-to-many edge to the target resource
	RelationHasMany RelationKind = "has-many"
)

// OnDeleteAction defines referential behavior when the target row is deleted
type OnDeleteAction string

const (
	// OnDeleteRestrict blocks deletion while references remain (default)
	OnDeleteRestrict OnDeleteAction = "restrict"
	// OnDeleteCascade deletes the referencing rows
	OnDeleteCascade OnDeleteAction = "cascade"
	// OnDeleteSetNull nulls the referencing column; requires a nullable field
	OnDeleteSetNull OnDeleteAction = "set-null"
)

// FieldKind is a coarse, underlying Go type category used for validation.
type FieldKind string

const (
	// FieldKindUnknown means the parser could not resolve the underlying type.
	FieldKindUnknown FieldKind = "unknown"
	// FieldKindString represents string and named string types.
	FieldKindString FieldKind = "string"
	// FieldKindBool represents bool and named bool types.
	FieldKindBool FieldKind = "bool"
	// FieldKindInt represents signed and unsigned integer types.
	FieldKindInt FieldKind = "int"
	// FieldKindFloat represents float32 and float64 types.
	FieldKindFloat FieldKind = "float"
	// FieldKindSlice represents slices and arrays.
	FieldKindSlice FieldKind = "slice"
	// FieldKindMap represents maps.
	FieldKindMap FieldKind = "map"
	// FieldKindStruct represents structs and known struct aliases.
	FieldKindStruct FieldKind = "struct"
)

// FieldTypeInfo records parser-derived semantic type metadata for a field.
type FieldTypeInfo struct {
	Syntax         string
	UnderlyingKind FieldKind
	PointerDepth   int
	NamedType      string
	IsResolved     bool
	IsStringLike   bool
	IsScalar       bool
	IsComparable   bool
	IsTime         bool
}

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

	// SpecFields contains every Go field name discovered on the resource's Spec
	// struct, including fields with no annotations. It is populated by parsers so
	// resource-level annotations can validate field references.
	SpecFields map[string]bool

	// Indexes holds multi-column indexes declared at the resource level
	// (+fabrica:index:fields=a,b). Single-column indexes stay on the field.
	Indexes []*CompositeIndex

	// Migration constrains what a generated migration may do to this table
	Migration MigrationPolicy

	// RawAnnotations contains unparsed annotation lines for debugging
	RawAnnotations []string
}

// NewResourceAnnotations creates a new ResourceAnnotations with defaults
func NewResourceAnnotations() *ResourceAnnotations {
	return &ResourceAnnotations{
		IsResource:  false,
		StorageMode: StorageModeGeneric,
		Fields:      make(map[string]*FieldAnnotations),
		SpecFields:  make(map[string]bool),
		Migration:   MigrationPolicyUnrestricted,
	}
}

// CompositeIndex describes a multi-column index on a resource's dedicated table
type CompositeIndex struct {
	// Name is an optional explicit index name; derived from fields when empty
	Name string

	// Fields lists the resource field names covered, in index order
	Fields []string

	// Unique makes this a unique index
	Unique bool

	// Type is the index type (defaults to btree)
	Type IndexType
}

// RelationConfig describes a foreign-key relation to another resource type
type RelationConfig struct {
	// Kind is the relation cardinality
	Kind RelationKind

	// Target is the referenced resource type name
	Target string

	// OnDelete is the referential action (defaults to restrict)
	OnDelete OnDeleteAction
}

// FieldAnnotations contains all annotations for a single field
type FieldAnnotations struct {
	// FieldName is the Go field name
	FieldName string

	// FieldType is the Go type syntax discovered by the parser when available.
	FieldType string

	// TypeInfo is parser-derived semantic type metadata when available.
	TypeInfo FieldTypeInfo

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

	// Nullable forces the column to permit NULL, overriding the inference
	// made from the Go struct tag (+fabrica:field:nullable)
	Nullable bool

	// NotNull forces the column to reject NULL, overriding the inference
	// made from the Go struct tag (+fabrica:field:notnull)
	NotNull bool

	// Size caps the stored width of a string column (0 = unset)
	Size int

	// Relation declares a foreign-key relation to another resource type
	Relation *RelationConfig

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

// ValidationError represents an annotation validation error with context
type ValidationError struct {
	File       string // Source file path
	Line       int    // Line number in source
	Field      string // Field name
	Annotation string // The annotation that failed validation
	Message    string // Error message
	Severity   string // "error" or "warning"
}

func (e *ValidationError) Error() string {
	// If we have file/line context, use it
	if e.File != "" && e.Line > 0 {
		if e.Severity == "warning" {
			return fmt.Sprintf("%s:%d: warning: %s: %s", e.File, e.Line, e.Field, e.Message)
		}
		return fmt.Sprintf("%s:%d: %s: %s", e.File, e.Line, e.Field, e.Message)
	}

	// Fallback to old format for backward compatibility
	if e.Annotation != "" {
		return fmt.Sprintf("invalid annotation %q: %s", e.Annotation, e.Message)
	}

	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}

	return e.Message
}

// IsError returns true if this is an error (not a warning)
func (e *ValidationError) IsError() bool {
	return e.Severity != "warning"
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
	if key, value, ok := strings.Cut(part, "="); ok {
		return key, value, true
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

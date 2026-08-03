// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"strings"
)

// Validate checks that annotations are semantically correct
//
// This performs validation beyond syntax checking, such as:
//   - Dedicated storage requires at least one field annotation
//   - Hashed/encrypted fields must be strings
//   - Conflicting annotations (e.g., immutable + default)
func Validate(annotations *ResourceAnnotations) error {
	// If not marked as a resource, no validation needed
	if !annotations.IsResource {
		return nil
	}

	// Dedicated storage validation
	if annotations.StorageMode == StorageModeDedicated {
		if err := validateDedicatedStorage(annotations); err != nil {
			return err
		}
	}

	// Composite indexes are a dedicated-table concept
	if err := validateCompositeIndexes(annotations); err != nil {
		return err
	}

	// Field-level validation
	for fieldName, fieldAnnotations := range annotations.Fields {
		if err := validateFieldAnnotations(fieldName, fieldAnnotations); err != nil {
			return err
		}

		if err := validateFieldStorageMode(fieldName, fieldAnnotations, annotations.StorageMode); err != nil {
			return err
		}
	}

	return nil
}

// validateCompositeIndexes checks resource-level multi-column indexes
func validateCompositeIndexes(annotations *ResourceAnnotations) error {
	if len(annotations.Indexes) == 0 {
		return nil
	}

	if annotations.StorageMode != StorageModeDedicated {
		return &ValidationError{
			Annotation: "+fabrica:index",
			Message:    "composite indexes require +fabrica:storage=dedicated (generic storage keeps spec in a single JSON column)",
		}
	}

	seen := make(map[string]bool)

	for _, idx := range annotations.Indexes {
		if len(idx.Fields) < 2 {
			return &ValidationError{
				Annotation: "+fabrica:index",
				Message: fmt.Sprintf(
					"composite index on %v covers a single column; use +fabrica:field:index on that field instead",
					idx.Fields),
			}
		}

		dupe := make(map[string]bool)
		for _, f := range idx.Fields {
			if dupe[f] {
				return &ValidationError{
					Annotation: "+fabrica:index",
					Message:    fmt.Sprintf("composite index lists field %q more than once", f),
				}
			}
			dupe[f] = true

			if _, ok := annotations.Fields[f]; !ok {
				// Not fatal: the field may carry no annotations of its own and
				// therefore never enters the Fields map. Emitters resolve names
				// against the struct, so only flag obviously empty names.
				if f == "" {
					return &ValidationError{
						Annotation: "+fabrica:index",
						Message:    "composite index contains an empty field name",
					}
				}
			}
		}

		if idx.Name != "" {
			if seen[idx.Name] {
				return &ValidationError{
					Annotation: "+fabrica:index",
					Message:    fmt.Sprintf("duplicate composite index name %q", idx.Name),
				}
			}
			seen[idx.Name] = true
		}
	}

	return nil
}

// validateFieldStorageMode rejects field annotations that only mean something
// on a dedicated table. Generic storage keeps spec/status in a single JSON
// column, so per-column intent has nowhere to land.
//
// Only the vocabulary introduced alongside composite indexes is checked here.
// The pre-existing annotations (index, unique, default, storage=…) stay lenient
// in generic mode for backward compatibility.
func validateFieldStorageMode(fieldName string, annotations *FieldAnnotations, mode StorageMode) error {
	if mode == StorageModeDedicated {
		return nil
	}

	var offender string
	switch {
	case annotations.Nullable:
		offender = "nullable"
	case annotations.NotNull:
		offender = "notnull"
	case annotations.Size > 0:
		offender = "size"
	case annotations.Relation != nil:
		offender = "relation"
	default:
		return nil
	}

	return &ValidationError{
		Field:      fieldName,
		Annotation: fmt.Sprintf("+fabrica:field:%s", offender),
		Message:    fmt.Sprintf("%s requires +fabrica:storage=dedicated", offender),
	}
}

// validateDedicatedStorage checks dedicated storage requirements
func validateDedicatedStorage(annotations *ResourceAnnotations) error {
	// Dedicated storage should have at least one field annotation
	// (otherwise, why not use generic storage?)
	if len(annotations.Fields) == 0 {
		return &ValidationError{
			Annotation: "+fabrica:storage=dedicated",
			Message:    "dedicated storage requires at least one field annotation to justify separate table",
		}
	}

	return nil
}

// validateFieldAnnotations checks field-level annotations for conflicts
func validateFieldAnnotations(fieldName string, annotations *FieldAnnotations) error {
	// Immutable + default can conflict (default set on update attempt)
	if annotations.Immutable && annotations.Default != "" {
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s", fieldName),
			Message:    "immutable fields should not have database defaults (set in code instead)",
		}
	}

	// Storage validation
	if annotations.Storage != nil {
		if err := validateStorageConfig(fieldName, annotations.Storage); err != nil {
			return err
		}
	}

	// Nullability must not be asserted both ways
	if annotations.Nullable && annotations.NotNull {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s", fieldName),
			Message:    "cannot be both nullable and notnull",
		}
	}

	// Index validation
	if annotations.Index != nil {
		if err := validateIndexConfig(fieldName, annotations.Index); err != nil {
			return err
		}
	}

	// Relation validation
	if annotations.Relation != nil {
		if err := validateRelationConfig(fieldName, annotations); err != nil {
			return err
		}
	}

	return nil
}

// validateRelationConfig checks a foreign-key relation declaration
func validateRelationConfig(fieldName string, annotations *FieldAnnotations) error {
	rel := annotations.Relation

	if rel.Target == "" {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation", fieldName),
			Message:    "relation requires a target resource type",
		}
	}

	if !isGoIdentifier(rel.Target) {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation", fieldName),
			Message:    fmt.Sprintf("relation target %q is not a valid Go type name", rel.Target),
		}
	}

	// SET NULL cannot apply to a column that refuses NULL.
	if rel.OnDelete == OnDeleteSetNull && annotations.NotNull {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation on-delete=set-null", fieldName),
			Message:    "on-delete=set-null conflicts with +fabrica:field:notnull",
		}
	}

	// An immutable column cannot be rewritten by a referential action.
	if rel.OnDelete == OnDeleteSetNull && annotations.Immutable {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation on-delete=set-null", fieldName),
			Message:    "on-delete=set-null conflicts with +fabrica:field:immutable",
		}
	}

	return nil
}

// isGoIdentifier reports whether s is a legal, exported-looking Go type name
func isGoIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// validateStorageConfig validates storage configuration
func validateStorageConfig(fieldName string, config *StorageConfig) error {
	switch config.Type {
	case StorageTypeHashed:
		if config.Hash == nil {
			return &ValidationError{
				Annotation: fmt.Sprintf("field %s storage=hashed", fieldName),
				Message:    "hashed storage missing hash configuration",
			}
		}
		return validateHashConfig(fieldName, config.Hash)

	case StorageTypeEncrypted:
		if config.Encryption == nil {
			return &ValidationError{
				Annotation: fmt.Sprintf("field %s storage=encrypted", fieldName),
				Message:    "encrypted storage missing encryption configuration",
			}
		}
		return validateEncryptionConfig(fieldName, config.Encryption)

	case StorageTypeDefault:
		return nil

	default:
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s storage", fieldName),
			Message:    fmt.Sprintf("unknown storage type: %s", config.Type),
		}
	}
}

// validateHashConfig validates hash configuration
func validateHashConfig(fieldName string, config *HashConfig) error {
	switch config.Algorithm {
	case HashAlgorithmBcrypt:
		if config.Cost < 4 || config.Cost > 31 {
			return &ValidationError{
				Annotation: fmt.Sprintf("field %s bcrypt cost", fieldName),
				Message:    fmt.Sprintf("bcrypt cost must be 4-31, got %d", config.Cost),
			}
		}

	case HashAlgorithmArgon2:
		if config.Cost < 1024 || config.Cost > 1048576 {
			return &ValidationError{
				Annotation: fmt.Sprintf("field %s argon2 memory", fieldName),
				Message:    fmt.Sprintf("argon2 memory must be 1024-1048576 KB, got %d", config.Cost),
			}
		}

	case HashAlgorithmSHA256:
		// No parameters to validate

	default:
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s hash algorithm", fieldName),
			Message:    fmt.Sprintf("unknown hash algorithm: %s", config.Algorithm),
		}
	}

	return nil
}

// validateEncryptionConfig validates encryption configuration
func validateEncryptionConfig(fieldName string, config *EncryptionConfig) error {
	// Validate algorithm
	validAlgorithms := map[string]bool{
		"aes128": true,
		"aes192": true,
		"aes256": true,
	}

	if !validAlgorithms[config.Algorithm] {
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s encryption algorithm", fieldName),
			Message:    fmt.Sprintf("unknown encryption algorithm %q, expected 'aes128', 'aes192', or 'aes256'", config.Algorithm),
		}
	}

	// Validate key source
	validKeySources := map[string]bool{
		"env":   true,
		"vault": true,
		"kms":   true,
	}

	if !validKeySources[config.KeySource] {
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s key source", fieldName),
			Message:    fmt.Sprintf("unknown key source %q, expected 'env', 'vault', or 'kms'", config.KeySource),
		}
	}

	return nil
}

// validateIndexConfig validates index configuration
func validateIndexConfig(fieldName string, config *IndexConfig) error {
	// Validate index type
	validTypes := map[IndexType]bool{
		IndexTypeBTree: true,
		IndexTypeGIN:   true,
		IndexTypeGiST:  true,
		IndexTypeHash:  true,
	}

	if !validTypes[config.Type] {
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s index type", fieldName),
			Message:    fmt.Sprintf("unknown index type: %s", config.Type),
		}
	}

	return nil
}

// ValidateForDatabase validates annotations against database capabilities
//
// Some features (like GIN indexes) are PostgreSQL-specific. This validates
// that annotations are compatible with the target database.
func ValidateForDatabase(annotations *ResourceAnnotations, dbDriver string) error {
	for fieldName, fieldAnnotations := range annotations.Fields {
		if fieldAnnotations.Index != nil {
			if err := validateIndexForDatabase(fieldName, fieldAnnotations.Index, dbDriver); err != nil {
				return err
			}
		}
	}

	// Composite indexes obey the same per-database index-type rules
	for _, idx := range annotations.Indexes {
		label := strings.Join(idx.Fields, ",")
		if err := validateIndexForDatabase(label, &IndexConfig{Type: idx.Type}, dbDriver); err != nil {
			return err
		}
	}

	return nil
}

// validateIndexForDatabase checks database-specific index support
func validateIndexForDatabase(fieldName string, config *IndexConfig, dbDriver string) error {
	switch dbDriver {
	case "sqlite3", "sqlite":
		// SQLite only supports B-tree indexes
		if config.Type != IndexTypeBTree {
			return &ValidationError{
				Annotation: fmt.Sprintf("field %s index=%s", fieldName, config.Type),
				Message:    fmt.Sprintf("SQLite only supports B-tree indexes, not %s", config.Type),
			}
		}

	case "mysql":
		// MySQL doesn't support GiST
		if config.Type == IndexTypeGiST {
			return &ValidationError{
				Annotation: fmt.Sprintf("field %s index=gist", fieldName),
				Message:    "MySQL does not support GiST indexes (PostgreSQL only)",
			}
		}

	case "postgres", "postgresql":
		// PostgreSQL supports all index types
		// No validation needed

	default:
		// Unknown database - allow all (fail at runtime if unsupported)
	}

	return nil
}

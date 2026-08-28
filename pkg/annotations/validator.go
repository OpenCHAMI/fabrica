// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// Validate checks that annotations are semantically correct
//
// This performs validation beyond syntax checking, such as:
//   - Dedicated storage requires at least one field annotation
//   - Hashed/encrypted fields must be strings
//   - Conflicting annotations (e.g., immutable + default)
func Validate(annotations *ResourceAnnotations) error {
	if annotations == nil {
		return &ValidationError{Message: "annotations must not be nil"}
	}

	// If not marked as a resource, no validation needed
	if !annotations.IsResource {
		return nil
	}

	if err := validateStorageMode(annotations.StorageMode); err != nil {
		return err
	}

	if err := validateMigrationPolicy(annotations.Migration); err != nil {
		return err
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
		if fieldAnnotations == nil {
			return &ValidationError{
				Field:   fieldName,
				Message: "field annotations must not be nil",
			}
		}

		if err := validateFieldAnnotations(fieldName, fieldAnnotations); err != nil {
			return err
		}

		if err := validateFieldStorageMode(fieldName, fieldAnnotations, annotations.StorageMode); err != nil {
			return err
		}
	}

	return nil
}

func validateStorageMode(mode StorageMode) error {
	switch mode {
	case "", StorageModeGeneric, StorageModeDedicated:
		return nil
	default:
		return &ValidationError{
			Annotation: "+fabrica:storage",
			Message:    fmt.Sprintf("unknown storage mode: %s", mode),
		}
	}
}

func validateMigrationPolicy(policy MigrationPolicy) error {
	switch policy {
	case "", MigrationPolicyUnrestricted, MigrationPolicyAdditiveOnly:
		return nil
	default:
		return &ValidationError{
			Annotation: "+fabrica:migration",
			Message:    fmt.Sprintf("unknown migration policy: %s", policy),
		}
	}
}

// validateCompositeIndexes checks resource-level multi-column indexes
func validateCompositeIndexes(annotations *ResourceAnnotations) error {
	if len(annotations.Indexes) == 0 {
		return validateFieldIndexNames(annotations.Fields, nil)
	}

	if annotations.StorageMode != StorageModeDedicated {
		return &ValidationError{
			Annotation: "+fabrica:index",
			Message:    "composite indexes require +fabrica:storage=dedicated (generic storage keeps spec in a single JSON column)",
		}
	}

	seen := make(map[string]bool)
	if err := validateFieldIndexNames(annotations.Fields, seen); err != nil {
		return err
	}

	for _, idx := range annotations.Indexes {
		if idx == nil {
			return &ValidationError{
				Annotation: "+fabrica:index",
				Message:    "composite index must not be nil",
			}
		}
		if err := validateIndexType("+fabrica:index", idx.Type); err != nil {
			return err
		}
		if idx.Name != "" && !isPortableIdentifier(idx.Name) {
			return &ValidationError{
				Annotation: "+fabrica:index",
				Message:    fmt.Sprintf("invalid index name %q", idx.Name),
			}
		}
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
			if !isExportedGoIdentifier(f) {
				return &ValidationError{
					Annotation: "+fabrica:index",
					Message:    fmt.Sprintf("composite index field %q is not a valid Go field name", f),
				}
			}
			if dupe[f] {
				return &ValidationError{
					Annotation: "+fabrica:index",
					Message:    fmt.Sprintf("composite index lists field %q more than once", f),
				}
			}
			dupe[f] = true

			if len(annotations.SpecFields) > 0 && !annotations.SpecFields[f] {
				return &ValidationError{
					Annotation: "+fabrica:index",
					Message:    fmt.Sprintf("composite index references unknown Spec field %q", f),
				}
			}
		}

		if idx.Name != "" {
			if seen[idx.Name] {
				return &ValidationError{
					Annotation: "+fabrica:index",
					Message:    fmt.Sprintf("duplicate index name %q", idx.Name),
				}
			}
			seen[idx.Name] = true
		}
	}

	return nil
}

func validateFieldIndexNames(fields map[string]*FieldAnnotations, seen map[string]bool) error {
	if seen == nil {
		seen = make(map[string]bool)
	}
	for fieldName, fieldAnnotations := range fields {
		if fieldAnnotations == nil || fieldAnnotations.Index == nil || fieldAnnotations.Index.Name == "" {
			continue
		}
		name := fieldAnnotations.Index.Name
		if !isPortableIdentifier(name) {
			return &ValidationError{
				Field:      fieldName,
				Annotation: fmt.Sprintf("field %s index", fieldName),
				Message:    fmt.Sprintf("invalid index name %q", name),
			}
		}
		if seen[name] {
			return &ValidationError{
				Field:      fieldName,
				Annotation: fmt.Sprintf("field %s index", fieldName),
				Message:    fmt.Sprintf("duplicate index name %q", name),
			}
		}
		seen[name] = true
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
	if annotations == nil {
		return &ValidationError{
			Field:   fieldName,
			Message: "field annotations must not be nil",
		}
	}

	// Immutable + default can conflict (default set on update attempt)
	if annotations.Immutable && annotations.Default != "" {
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s", fieldName),
			Message:    "immutable fields should not have database defaults (set in code instead)",
		}
	}

	// Storage validation
	if annotations.Storage != nil {
		if err := validateStorageConfig(fieldName, annotations); err != nil {
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

	if annotations.Size < 0 || annotations.Size > 65535 {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s size", fieldName),
			Message:    fmt.Sprintf("field size must be 1-65535 when set, got %d", annotations.Size),
		}
	}
	if annotations.Size > 0 {
		ok, desc := isStringLikeFieldWithDiagnostic(annotations)
		if !ok {
			return &ValidationError{
				Field:      fieldName,
				Annotation: fmt.Sprintf("field %s size", fieldName),
				Message:    fmt.Sprintf("size requires a string field, got %s", desc),
			}
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
	if rel == nil {
		return nil
	}

	if rel.Target == "" {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation", fieldName),
			Message:    "relation requires a target resource type",
		}
	}

	if !isExportedGoIdentifier(rel.Target) {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation", fieldName),
			Message:    fmt.Sprintf("relation target %q is not a valid Go type name", rel.Target),
		}
	}

	switch rel.Kind {
	case RelationBelongsTo, RelationHasMany:
	default:
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation", fieldName),
			Message:    fmt.Sprintf("unknown relation kind: %s", rel.Kind),
		}
	}

	switch rel.OnDelete {
	case "", OnDeleteRestrict, OnDeleteCascade, OnDeleteSetNull:
	default:
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation on-delete", fieldName),
			Message:    fmt.Sprintf("unknown on-delete action: %s", rel.OnDelete),
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

	if rel.OnDelete == OnDeleteSetNull && !annotations.Nullable {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s relation on-delete=set-null", fieldName),
			Message:    "on-delete=set-null requires +fabrica:field:nullable",
		}
	}

	return nil
}

func isExportedGoIdentifier(s string) bool {
	return token.IsIdentifier(s) && ast.IsExported(s)
}

// validateStorageConfig validates storage configuration
func validateStorageConfig(fieldName string, annotations *FieldAnnotations) error {
	config := annotations.Storage
	switch config.Type {
	case StorageTypeHashed:
		if ok, desc := isStringLikeFieldWithDiagnostic(annotations); !ok {
			return &ValidationError{
				Field:      fieldName,
				Annotation: fmt.Sprintf("field %s storage", fieldName),
				Message:    fmt.Sprintf("hashed storage requires a string field, got %s", desc),
			}
		}
		if config.Encryption != nil {
			return &ValidationError{
				Field:      fieldName,
				Annotation: fmt.Sprintf("field %s storage", fieldName),
				Message:    "hashed storage must not include encryption config",
			}
		}
		if config.Hash == nil {
			return &ValidationError{
				Annotation: fmt.Sprintf("field %s storage=hashed", fieldName),
				Message:    "hashed storage missing hash configuration",
			}
		}
		return validateHashConfig(fieldName, config.Hash)

	case StorageTypeEncrypted:
		if ok, desc := isStringLikeFieldWithDiagnostic(annotations); !ok {
			return &ValidationError{
				Field:      fieldName,
				Annotation: fmt.Sprintf("field %s storage", fieldName),
				Message:    fmt.Sprintf("encrypted storage requires a string field, got %s", desc),
			}
		}
		if config.Hash != nil {
			return &ValidationError{
				Field:      fieldName,
				Annotation: fmt.Sprintf("field %s storage", fieldName),
				Message:    "encrypted storage must not include hash config",
			}
		}
		if config.Encryption == nil {
			return &ValidationError{
				Annotation: fmt.Sprintf("field %s storage=encrypted", fieldName),
				Message:    "encrypted storage missing encryption configuration",
			}
		}
		return validateEncryptionConfig(fieldName, config.Encryption)

	case StorageTypeDefault:
		if config.Hash != nil || config.Encryption != nil {
			return &ValidationError{
				Field:      fieldName,
				Annotation: fmt.Sprintf("field %s storage", fieldName),
				Message:    "default storage must not include hash or encryption config",
			}
		}
		return nil

	default:
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s storage", fieldName),
			Message:    fmt.Sprintf("unknown storage type: %s", config.Type),
		}
	}
}

func isStringLikeField(annotations *FieldAnnotations) bool {
	if annotations.TypeInfo.Syntax != "" {
		// If we have type info but it is unresolved, treat as non-string.
		// Callers should distinguish "unresolved" from "non-string" for diagnostics.
		if !annotations.TypeInfo.IsResolved {
			return false
		}
		return annotations.TypeInfo.IsStringLike
	}

	switch annotations.FieldType {
	case "", "string", "*string":
		return true
	default:
		return false
	}
}

func fieldTypeDescription(annotations *FieldAnnotations) string {
	if annotations.TypeInfo.Syntax != "" {
		if !annotations.TypeInfo.IsResolved {
			return fmt.Sprintf("%s (unresolved type; ensure the package is available in the module cache)", annotations.TypeInfo.Syntax)
		}
		return annotations.TypeInfo.Syntax
	}
	if annotations.FieldType != "" {
		return annotations.FieldType
	}
	return "unknown"
}

// isStringLikeFieldWithDiagnostic returns whether the field is string-like and,
// if not, a more descriptive message distinguishing between unresolved types
// and definitively non-string types.
func isStringLikeFieldWithDiagnostic(annotations *FieldAnnotations) (bool, string) {
	if annotations.TypeInfo.Syntax != "" {
		if !annotations.TypeInfo.IsResolved {
			return false, fmt.Sprintf("%s (unresolved type; ensure the package is available in the module cache)", annotations.TypeInfo.Syntax)
		}
		if !annotations.TypeInfo.IsStringLike {
			return false, annotations.TypeInfo.Syntax
		}
		return true, annotations.TypeInfo.Syntax
	}

	switch annotations.FieldType {
	case "", "string", "*string":
		return true, annotations.FieldType
	default:
		return false, annotations.FieldType
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
	if config == nil {
		return &ValidationError{
			Annotation: fmt.Sprintf("field %s index", fieldName),
			Message:    "index config must not be nil",
		}
	}
	if config.Name != "" && !isPortableIdentifier(config.Name) {
		return &ValidationError{
			Field:      fieldName,
			Annotation: fmt.Sprintf("field %s index", fieldName),
			Message:    fmt.Sprintf("invalid index name %q", config.Name),
		}
	}
	return validateIndexType(fmt.Sprintf("field %s index type", fieldName), config.Type)
}

func validateIndexType(annotation string, indexType IndexType) error {
	// Validate index type
	validTypes := map[IndexType]bool{
		IndexTypeBTree: true,
		IndexTypeGIN:   true,
		IndexTypeGiST:  true,
		IndexTypeHash:  true,
	}

	if !validTypes[indexType] {
		return &ValidationError{
			Annotation: annotation,
			Message:    fmt.Sprintf("unknown index type: %s", indexType),
		}
	}

	return nil
}

func isPortableIdentifier(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '_':
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

// ValidateForDatabase validates annotations against database capabilities
//
// Some features (like GIN indexes) are PostgreSQL-specific. This validates
// that annotations are compatible with the target database.
func ValidateForDatabase(annotations *ResourceAnnotations, dbDriver string) error {
	if err := Validate(annotations); err != nil {
		return err
	}

	switch dbDriver {
	case "postgres", "postgresql", "mysql", "mariadb", "sqlite", "sqlite3":
	default:
		return &ValidationError{
			Annotation: "database",
			Message:    fmt.Sprintf("unknown database driver: %s", dbDriver),
		}
	}

	for fieldName, fieldAnnotations := range annotations.Fields {
		if fieldAnnotations == nil {
			return &ValidationError{
				Field:   fieldName,
				Message: "field annotations must not be nil",
			}
		}
		if fieldAnnotations.Index != nil {
			if err := validateIndexConfig(fieldName, fieldAnnotations.Index); err != nil {
				return err
			}
			if err := validateIndexForDatabase(fieldName, fieldAnnotations.Index, dbDriver); err != nil {
				return err
			}
		}
	}

	// Composite indexes obey the same per-database index-type rules
	for _, idx := range annotations.Indexes {
		if idx == nil {
			return &ValidationError{
				Annotation: "+fabrica:index",
				Message:    "composite index must not be nil",
			}
		}
		if err := validateIndexType("+fabrica:index", idx.Type); err != nil {
			return err
		}
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

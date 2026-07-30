// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import "reflect"

// Dialect identifies a database with a proven dedicated-storage implementation.
type Dialect uint8

const (
	// DialectUnknown represents a missing or unsupported database dialect.
	DialectUnknown Dialect = iota
	// DialectPostgreSQL selects PostgreSQL-specific dedicated-storage capabilities.
	DialectPostgreSQL
	// DialectSQLite selects SQLite-compatible dedicated-storage capabilities.
	DialectSQLite
)

// ResourceStorageKind selects the authoritative persistence model for a resource.
type ResourceStorageKind uint8

const (
	// ResourceStorageUnknown represents an unresolved resource storage selection.
	ResourceStorageUnknown ResourceStorageKind = iota
	// ResourceStorageGeneric selects the shared JSON resource entity.
	ResourceStorageGeneric
	// ResourceStorageDedicated selects a resource-specific Ent entity.
	ResourceStorageDedicated
)

// FieldKind identifies a Go field type supported by dedicated Ent storage.
type FieldKind uint8

const (
	// FieldKindUnknown represents a Go type outside the supported field set.
	FieldKindUnknown FieldKind = iota
	// FieldKindString represents string fields.
	FieldKindString
	// FieldKindBool represents boolean fields.
	FieldKindBool
	// FieldKindInt represents platform-sized integer fields.
	FieldKindInt
	// FieldKindInt64 represents signed 64-bit integer fields.
	FieldKindInt64
	// FieldKindFloat64 represents 64-bit floating-point fields.
	FieldKindFloat64
	// FieldKindTime represents time.Time fields.
	FieldKindTime
	// FieldKindStringSlice represents []string fields.
	FieldKindStringSlice
)

// Optionality describes whether a generated Ent field is required, optional, or nillable.
type Optionality uint8

const (
	// OptionalityUnknown represents an unresolved field-presence contract.
	OptionalityUnknown Optionality = iota
	// OptionalityRequired represents a non-pointer field with required validation.
	OptionalityRequired
	// OptionalityOptional represents a non-pointer field that may use its zero value.
	OptionalityOptional
	// OptionalityNillable represents a supported pointer field that preserves absence.
	OptionalityNillable
)

// TransformKind identifies the storage transformation applied before persistence.
type TransformKind uint8

const (
	// TransformUnknown represents an unresolved or unsupported storage transformation.
	TransformUnknown TransformKind = iota
	// TransformStandard persists the field without a storage transformation.
	TransformStandard
	// TransformBcrypt persists a bcrypt hash instead of plaintext.
	TransformBcrypt
)

// IndexKind identifies a validated database index method.
type IndexKind uint8

const (
	// IndexUnknown represents an unresolved or unsupported index method.
	IndexUnknown IndexKind = iota
	// IndexNone requests no standalone database index.
	IndexNone
	// IndexBTree requests the portable default B-tree index.
	IndexBTree
	// IndexGIN requests PostgreSQL GIN indexing for []string.
	IndexGIN
	// IndexGiST represents the recognized but unsupported GiST method.
	IndexGiST
	// IndexHash requests PostgreSQL hash indexing for scalar equality fields.
	IndexHash
)

// SourcePosition locates the declaration and directive that produced a storage decision.
type SourcePosition struct {
	Filename  string
	Line      int
	Column    int
	TypeName  string
	FieldName string
	Directive string
}

// FieldType retains the normalized field kind and exact reflected Go shape.
type FieldType struct {
	Kind    FieldKind
	pointer bool
	goType  reflect.Type
}

// GoType returns the exact reflected type, including a supported pointer wrapper.
func (t FieldType) GoType() reflect.Type {
	return t.goType
}

// Pointer reports whether the source field uses a supported pointer type.
func (t FieldType) Pointer() bool {
	return t.pointer
}

// StorageTransform contains the validated persistence transform and its bounded parameters.
type StorageTransform struct {
	Kind       TransformKind
	BcryptCost int
}

// ResolvedFieldStorage is the complete storage decision for one Spec field.
type ResolvedFieldStorage struct {
	Source      SourcePosition
	GoName      string
	JSONName    string
	Type        FieldType
	Optionality Optionality
	Transform   StorageTransform
	Default     DefaultValue
	Index       IndexKind
	Dialect     Dialect
	Sensitive   bool
	Immutable   bool
	Unique      bool
}

// ResolvedResourceStorage is the immutable, dialect-specific storage contract for a resource.
type ResolvedResourceStorage struct {
	Source  SourcePosition
	Name    string
	Storage ResourceStorageKind
	Dialect Dialect
	Fields  []ResolvedFieldStorage
}

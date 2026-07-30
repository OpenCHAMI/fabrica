// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// ResolveStorageIntent parses source and returns the validated storage contract for one resource.
func ResolveStorageIntent(filename, resourceName string, dialect Dialect) (*ResolvedResourceStorage, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		globalCache.Invalidate(filename)
		return nil, fmt.Errorf("read file %s: %w", filename, err)
	}
	cacheKey := resolvedCacheKey{filename: filename, resourceName: resourceName, dialect: dialect}
	if cached, ok := globalCache.getStorage(cacheKey, source); ok {
		return cached, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file %s: %w", filename, err)
	}
	declarations := findTypeDeclarations(file, resourceName)
	resourceSource := SourcePosition{Filename: filename, TypeName: resourceName, Directive: "database dialect"}
	if resource, exists := declarations[resourceName]; exists {
		resourceSource = sourceAt(fset.Position(resource.spec.Pos()), SourcePosition{TypeName: resourceName, Directive: "database dialect"})
	}
	if dialect != DialectPostgreSQL && dialect != DialectSQLite {
		return nil, capabilityError(resourceSource, CapabilityDialect, "database dialect is unknown")
	}

	raw, err := resolveResourceDeclarations(declarations, resourceName, fset)
	if err != nil {
		return nil, err
	}
	resolved := &ResolvedResourceStorage{
		Source: resourceSource, Name: resourceName, Storage: resolvedStorageKind(raw.StorageMode), Dialect: dialect,
	}
	spec, exists := declarations[resourceName+"Spec"]
	if !exists {
		globalCache.setStorage(cacheKey, source, resolved)
		return resolved, nil
	}
	structType, ok := spec.spec.Type.(*ast.StructType)
	if !ok {
		return nil, capabilityError(
			sourceAt(fset.Position(spec.spec.Pos()), SourcePosition{TypeName: spec.spec.Name.Name, Directive: "field declaration"}),
			CapabilityFieldType,
			"Spec declaration must be a struct",
		)
	}
	resolver := storageResolver{dialect: dialect, typeName: spec.spec.Name.Name, fset: fset}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			intent, resolveErr := resolver.resolveField(field, name.Name, raw.Fields[name.Name])
			if resolveErr != nil {
				return nil, resolveErr
			}
			resolved.Fields = append(resolved.Fields, intent)
		}
	}
	globalCache.setStorage(cacheKey, source, resolved)
	return resolved, nil
}

type storageResolver struct {
	dialect  Dialect
	typeName string
	fset     *token.FileSet
}

func (r storageResolver) resolveField(field *ast.Field, fieldName string, raw *FieldAnnotations) (ResolvedFieldStorage, error) {
	source := sourceAt(r.fset.Position(field.Pos()), SourcePosition{TypeName: r.typeName, FieldName: fieldName, Directive: "field declaration"})
	fieldType, err := fieldTypeFromAST(field.Type, source)
	if err != nil {
		return ResolvedFieldStorage{}, err
	}
	intent := ResolvedFieldStorage{
		Source: source, GoName: fieldName, JSONName: jsonFieldName(field, fieldName), Type: fieldType,
		Optionality: fieldOptionality(field, fieldType), Transform: StorageTransform{Kind: TransformStandard},
		Index: IndexNone, Dialect: r.dialect,
	}
	if raw == nil {
		return intent, nil
	}
	intent.Sensitive = raw.Sensitive
	intent.Immutable = raw.Immutable
	intent.Unique = raw.Unique
	if raw.Storage != nil {
		intent.Source.Directive = annotationWithPrefix(raw.RawAnnotations, "+fabrica:field:storage")
		transform, transformErr := resolveTransform(fieldType, raw.Storage, intent.Source)
		if transformErr != nil {
			return ResolvedFieldStorage{}, transformErr
		}
		intent.Transform = transform
	}
	if raw.Index != nil {
		intent.Source.Directive = annotationWithPrefix(raw.RawAnnotations, "+fabrica:field:index")
		index, indexErr := r.resolveIndex(fieldType, raw.Index, intent.Source)
		if indexErr != nil {
			return ResolvedFieldStorage{}, indexErr
		}
		intent.Index = index
	}
	if hasRawDefault(raw) {
		defaultSource := defaultDirectiveSource(field, r.fset, r.typeName, fieldName, raw.Default)
		if intent.Immutable {
			return ResolvedFieldStorage{}, defaultError(
				defaultSource, fieldType.Kind, DefaultErrorConflict,
				"immutable fields cannot have database defaults", nil,
			)
		}
		if intent.Transform.Kind != TransformStandard {
			return ResolvedFieldStorage{}, defaultError(
				defaultSource, fieldType.Kind, DefaultErrorConflict,
				"transformed fields cannot have database defaults", nil,
			)
		}
		defaultValue, defaultErr := parseDefaultValue(fieldType, raw.Default, defaultSource)
		if defaultErr != nil {
			return ResolvedFieldStorage{}, defaultErr
		}
		intent.Default = defaultValue
	}
	return intent, nil
}

func resolveTransform(fieldType FieldType, raw *StorageConfig, source SourcePosition) (StorageTransform, error) {
	switch raw.Type {
	case StorageTypeDefault:
		return StorageTransform{Kind: TransformStandard}, nil
	case StorageTypeHashed:
		if raw.Hash == nil || raw.Hash.Algorithm != HashAlgorithmBcrypt {
			return StorageTransform{}, capabilityError(source, CapabilityTransform, "only bcrypt hashing is supported")
		}
		if fieldType.Kind != FieldKindString {
			return StorageTransform{}, capabilityError(source, CapabilityTransform, "bcrypt hashing requires a string field")
		}
		return StorageTransform{Kind: TransformBcrypt, BcryptCost: raw.Hash.Cost}, nil
	case StorageTypeEncrypted:
		return StorageTransform{}, capabilityError(source, CapabilityTransform, "encrypted storage is not supported")
	default:
		return StorageTransform{}, capabilityError(source, CapabilityTransform, "storage transform is not supported")
	}
}

func (r storageResolver) resolveIndex(fieldType FieldType, raw *IndexConfig, source SourcePosition) (IndexKind, error) {
	switch raw.Type {
	case IndexTypeBTree:
		return IndexBTree, nil
	case IndexTypeGIN:
		if r.dialect == DialectSQLite {
			return IndexUnknown, capabilityError(source, CapabilityIndex, "SQLite supports only B-tree indexes")
		}
		if fieldType.Kind != FieldKindStringSlice {
			return IndexUnknown, capabilityError(source, CapabilityIndex, "PostgreSQL GIN indexes require a []string field")
		}
		return IndexGIN, nil
	case IndexTypeGiST:
		if r.dialect == DialectSQLite {
			return IndexUnknown, capabilityError(source, CapabilityIndex, "SQLite supports only B-tree indexes")
		}
		return IndexUnknown, capabilityError(source, CapabilityIndex, "PostgreSQL GiST indexes are not supported for the current field types")
	case IndexTypeHash:
		if r.dialect == DialectSQLite {
			return IndexUnknown, capabilityError(source, CapabilityIndex, "SQLite supports only B-tree indexes")
		}
		switch fieldType.Kind {
		case FieldKindString, FieldKindBool, FieldKindInt, FieldKindInt64, FieldKindFloat64, FieldKindTime:
			return IndexHash, nil
		case FieldKindStringSlice, FieldKindUnknown:
			return IndexUnknown, capabilityError(source, CapabilityIndex, "PostgreSQL hash indexes require a scalar equality field")
		default:
			return IndexUnknown, capabilityError(source, CapabilityIndex, "PostgreSQL hash index field type is not supported")
		}
	default:
		return IndexUnknown, capabilityError(source, CapabilityIndex, "index type is not supported")
	}
}

func resolvedStorageKind(mode StorageMode) ResourceStorageKind {
	switch mode {
	case StorageModeGeneric:
		return ResourceStorageGeneric
	case StorageModeDedicated:
		return ResourceStorageDedicated
	default:
		return ResourceStorageUnknown
	}
}

func fieldOptionality(field *ast.Field, fieldType FieldType) Optionality {
	if fieldType.Pointer() {
		return OptionalityNillable
	}
	if field.Tag != nil {
		tag, err := strconv.Unquote(field.Tag.Value)
		if err == nil && strings.Contains(reflect.StructTag(tag).Get("validate"), "required") {
			return OptionalityRequired
		}
	}
	return OptionalityOptional
}

func jsonFieldName(field *ast.Field, fallback string) string {
	if field.Tag == nil {
		return fallback
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return fallback
	}
	name := strings.Split(reflect.StructTag(tag).Get("json"), ",")[0]
	if name == "" || name == "-" {
		return fallback
	}
	return name
}

func sourceAt(position token.Position, source SourcePosition) SourcePosition {
	source.Filename = position.Filename
	source.Line = position.Line
	source.Column = position.Column
	return source
}

func annotationWithPrefix(annotations []string, prefix string) string {
	for _, annotation := range annotations {
		if strings.HasPrefix(annotation, prefix) {
			return annotation
		}
	}
	return prefix
}

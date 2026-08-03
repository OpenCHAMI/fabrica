// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// ParseResourceAnnotations extracts Fabrica annotations from a type declaration
//
// This parses annotations from the type's doc comments and field comments.
// The docComments parameter should be the GenDecl.Doc if available, or typeSpec.Doc otherwise.
//
// Example:
//
//	genDecl := ... // *ast.GenDecl from parsing Go code
//	typeSpec := genDecl.Specs[0].(*ast.TypeSpec)
//	annotations, err := ParseResourceAnnotations(typeSpec, genDecl.Doc)
func ParseResourceAnnotations(typeSpec *ast.TypeSpec, docComments *ast.CommentGroup) (*ResourceAnnotations, error) {
	result := NewResourceAnnotations()

	// Parse type-level annotations from doc comments
	// Try the provided docComments first (from GenDecl), then fall back to typeSpec.Doc
	comments := docComments
	if comments == nil {
		comments = typeSpec.Doc
	}

	if comments != nil {
		for _, comment := range comments.List {
			line := CleanAnnotationLine(comment.Text)
			if !IsFabricaAnnotation(line) {
				continue
			}

			result.RawAnnotations = append(result.RawAnnotations, line)

			if err := parseResourceLevelAnnotation(result, line); err != nil {
				return nil, err
			}
		}
	}

	// Parse field-level annotations if this is a struct
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return result, nil
	}

	for _, field := range structType.Fields.List {
		// Skip fields without names (embedded types)
		if len(field.Names) == 0 {
			continue
		}

		fieldName := field.Names[0].Name

		// Parse annotations from field comments
		if field.Doc != nil {
			fieldAnnotations := NewFieldAnnotations(fieldName)

			for _, comment := range field.Doc.List {
				line := CleanAnnotationLine(comment.Text)
				if !IsFabricaAnnotation(line) {
					continue
				}

				fieldAnnotations.RawAnnotations = append(fieldAnnotations.RawAnnotations, line)

				if err := parseFieldLevelAnnotation(fieldAnnotations, line); err != nil {
					return nil, &ParseError{Line: line, Message: err.Error()}
				}
			}

			// Only store if field has annotations
			if len(fieldAnnotations.RawAnnotations) > 0 {
				result.Fields[fieldName] = fieldAnnotations
			}
		}
	}

	return result, nil
}

// parseResourceLevelAnnotation processes a single resource-level annotation
func parseResourceLevelAnnotation(result *ResourceAnnotations, annotation string) error {
	parts := ParseAnnotationValue(annotation)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "resource":
		result.IsResource = true
		return nil

	case "index":
		return parseCompositeIndexAnnotation(result, parts[1:], annotation)

	default:
		// Try to parse as key=value format (e.g., storage=dedicated)
		key, value, hasValue := ParseKeyValue(parts[0])
		if !hasValue {
			return nil
		}

		if key == "migration" {
			switch MigrationPolicy(value) {
			case MigrationPolicyUnrestricted, MigrationPolicyAdditiveOnly:
				result.Migration = MigrationPolicy(value)
			default:
				return &ParseError{
					Line:    annotation,
					Message: fmt.Sprintf("unknown migration policy %q, expected 'unrestricted' or 'additive-only'", value),
				}
			}
			return nil
		}

		if key == "storage" {
			switch StorageMode(value) {
			case StorageModeGeneric:
				result.StorageMode = StorageModeGeneric
			case StorageModeDedicated:
				result.StorageMode = StorageModeDedicated
			default:
				return &ParseError{
					Line:    annotation,
					Message: fmt.Sprintf("unknown storage mode %q, expected 'generic' or 'dedicated'", value),
				}
			}
			return nil
		}

		return nil
	}
}

// parseFieldLevelAnnotation processes a single field-level annotation
func parseFieldLevelAnnotation(result *FieldAnnotations, annotation string) error {
	parts := ParseAnnotationValue(annotation)
	if len(parts) == 0 {
		return nil
	}

	// All field annotations start with "field:"
	if parts[0] != "field" {
		return &ParseError{
			Line:    annotation,
			Message: "field annotations must start with +fabrica:field:",
		}
	}

	if len(parts) < 2 {
		return &ParseError{
			Line:    annotation,
			Message: "field annotation requires a directive after 'field:'",
		}
	}

	directive := parts[1]

	switch {
	case directive == "sensitive":
		result.Sensitive = true
		return nil

	case directive == "immutable":
		result.Immutable = true
		return nil

	case directive == "unique":
		result.Unique = true
		return nil

	case directive == "nullable":
		result.Nullable = true
		return nil

	case directive == "notnull":
		result.NotNull = true
		return nil

	case strings.HasPrefix(directive, "size"):
		return parseSizeAnnotation(result, parts[1:], annotation)

	case strings.HasPrefix(directive, "relation"):
		return parseRelationAnnotation(result, parts[1:], annotation)

	case strings.HasPrefix(directive, "storage"):
		return parseStorageAnnotation(result, parts[1:], annotation)

	case strings.HasPrefix(directive, "index"):
		return parseIndexAnnotation(result, parts[1:], annotation)

	case strings.HasPrefix(directive, "default"):
		return parseDefaultAnnotation(result, parts[1:], annotation)

	default:
		// Unknown field annotation - ignore for forward compatibility
		return nil
	}
}

// parseStorageAnnotation parses +fabrica:field:storage=<type>:<details>
//
// Examples:
//   - +fabrica:field:storage=hashed:bcrypt:cost=12
//   - +fabrica:field:storage=encrypted:aes256:key=env
func parseStorageAnnotation(result *FieldAnnotations, parts []string, fullAnnotation string) error {
	if len(parts) == 0 {
		return fmt.Errorf("storage annotation requires parameters")
	}

	// Parse: storage=<type>
	key, value, hasValue := ParseKeyValue(parts[0])
	if !hasValue || key != "storage" {
		return fmt.Errorf("expected format: storage=<type>")
	}

	result.Storage = &StorageConfig{
		Type: StorageType(value),
	}

	switch result.Storage.Type {
	case StorageTypeHashed:
		return parseHashedStorage(result.Storage, parts[1:], fullAnnotation)
	case StorageTypeEncrypted:
		return parseEncryptedStorage(result.Storage, parts[1:], fullAnnotation)
	case StorageTypeDefault:
		return nil
	default:
		return fmt.Errorf("unknown storage type %q, expected 'hashed', 'encrypted', or 'default'", value)
	}
}

// parseHashedStorage parses hashed storage parameters
//
// Format: +fabrica:field:storage=hashed:bcrypt:cost=12
func parseHashedStorage(config *StorageConfig, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("hashed storage requires algorithm: storage=hashed:<algorithm>")
	}

	config.Hash = &HashConfig{
		Algorithm: HashAlgorithm(parts[0]),
	}

	// Parse algorithm-specific parameters
	switch config.Hash.Algorithm {
	case HashAlgorithmBcrypt:
		// Default bcrypt cost
		config.Hash.Cost = 12

		// Parse cost parameter if provided
		if len(parts) > 1 {
			for _, param := range parts[1:] {
				key, value, hasValue := ParseKeyValue(param)
				if !hasValue {
					continue
				}

				if key == "cost" {
					cost, err := ParseIntValue(value, 4, 31)
					if err != nil {
						return fmt.Errorf("bcrypt cost: %w", err)
					}
					config.Hash.Cost = cost
				}
			}
		}
		return nil

	case HashAlgorithmArgon2:
		// Default argon2 parameters
		config.Hash.Cost = 65536 // 64MB memory

		if len(parts) > 1 {
			for _, param := range parts[1:] {
				key, value, hasValue := ParseKeyValue(param)
				if !hasValue {
					continue
				}

				if key == "memory" {
					memory, err := ParseIntValue(value, 1024, 1048576)
					if err != nil {
						return fmt.Errorf("argon2 memory: %w", err)
					}
					config.Hash.Cost = memory
				}
			}
		}
		return nil

	case HashAlgorithmSHA256:
		// SHA256 has no cost parameter
		return nil

	default:
		return fmt.Errorf("unknown hash algorithm %q, expected 'bcrypt', 'argon2', or 'sha256'", config.Hash.Algorithm)
	}
}

// parseEncryptedStorage parses encrypted storage parameters
//
// Format: +fabrica:field:storage=encrypted:aes256:key=env
func parseEncryptedStorage(config *StorageConfig, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("encrypted storage requires algorithm: storage=encrypted:<algorithm>")
	}

	config.Encryption = &EncryptionConfig{
		Algorithm: parts[0],
		KeySource: "env", // default
	}

	// Parse key source if provided
	if len(parts) > 1 {
		for _, param := range parts[1:] {
			key, value, hasValue := ParseKeyValue(param)
			if !hasValue {
				continue
			}

			if key == "key" {
				config.Encryption.KeySource = value
			}
		}
	}

	return nil
}

// parseIndexAnnotation parses +fabrica:field:index or +fabrica:field:index=<type>
//
// Examples:
//   - +fabrica:field:index
//   - +fabrica:field:index=gin
func parseIndexAnnotation(result *FieldAnnotations, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("index annotation missing")
	}

	result.Index = &IndexConfig{
		Type: IndexTypeBTree, // default
	}

	key, value, hasValue := ParseKeyValue(parts[0])
	if key != "index" {
		return fmt.Errorf("expected 'index' key")
	}

	if hasValue {
		switch IndexType(value) {
		case IndexTypeBTree, IndexTypeGIN, IndexTypeGiST, IndexTypeHash:
			result.Index.Type = IndexType(value)
		default:
			return fmt.Errorf("unknown index type %q, expected 'btree', 'gin', 'gist', or 'hash'", value)
		}
	}

	// Trailing modifiers: unique and name=<identifier>
	for _, param := range parts[1:] {
		pKey, pValue, pHasValue := ParseKeyValue(param)
		switch {
		case pKey == "unique" && !pHasValue:
			result.Index.Unique = true
		case pKey == "name" && pHasValue:
			result.Index.Name = pValue
		default:
			return fmt.Errorf("unknown index modifier %q, expected 'unique' or 'name=<identifier>'", param)
		}
	}

	return nil
}

// parseCompositeIndexAnnotation parses a resource-level multi-column index.
//
// Format: +fabrica:index:fields=<f1,f2>[:name=<identifier>][:unique][:type=<type>]
func parseCompositeIndexAnnotation(result *ResourceAnnotations, parts []string, annotation string) error {
	idx := &CompositeIndex{Type: IndexTypeBTree}

	for _, param := range parts {
		key, value, hasValue := ParseKeyValue(param)
		switch {
		case key == "fields" && hasValue:
			for _, f := range strings.Split(value, ",") {
				if f = strings.TrimSpace(f); f != "" {
					idx.Fields = append(idx.Fields, f)
				}
			}

		case key == "name" && hasValue:
			idx.Name = value

		case key == "unique" && !hasValue:
			idx.Unique = true

		case key == "type" && hasValue:
			switch IndexType(value) {
			case IndexTypeBTree, IndexTypeGIN, IndexTypeGiST, IndexTypeHash:
				idx.Type = IndexType(value)
			default:
				return &ParseError{
					Line:    annotation,
					Message: fmt.Sprintf("unknown index type %q, expected 'btree', 'gin', 'gist', or 'hash'", value),
				}
			}

		default:
			return &ParseError{
				Line:    annotation,
				Message: fmt.Sprintf("unknown composite index parameter %q, expected 'fields=', 'name=', 'unique', or 'type='", param),
			}
		}
	}

	if len(idx.Fields) == 0 {
		return &ParseError{
			Line:    annotation,
			Message: "composite index requires fields=<f1,f2>",
		}
	}

	result.Indexes = append(result.Indexes, idx)
	return nil
}

// parseSizeAnnotation parses +fabrica:field:size=<n>
func parseSizeAnnotation(result *FieldAnnotations, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("size annotation missing")
	}

	key, value, hasValue := ParseKeyValue(parts[0])
	if !hasValue || key != "size" {
		return fmt.Errorf("expected format: size=<n>")
	}

	size, err := ParseIntValue(value, 1, 65535)
	if err != nil {
		return fmt.Errorf("field size: %w", err)
	}

	result.Size = size
	return nil
}

// parseRelationAnnotation parses a foreign-key relation to another resource.
//
// Format: +fabrica:field:relation=<belongs-to|has-many>:<Target>[:on-delete=<action>]
func parseRelationAnnotation(result *FieldAnnotations, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("relation annotation missing")
	}

	key, value, hasValue := ParseKeyValue(parts[0])
	if !hasValue || key != "relation" {
		return fmt.Errorf("expected format: relation=<belongs-to|has-many>:<Target>")
	}

	kind := RelationKind(value)
	switch kind {
	case RelationBelongsTo, RelationHasMany:
	default:
		return fmt.Errorf("unknown relation kind %q, expected 'belongs-to' or 'has-many'", value)
	}

	if len(parts) < 2 {
		return fmt.Errorf("relation requires a target resource type: relation=%s:<Target>", value)
	}

	rel := &RelationConfig{
		Kind:     kind,
		Target:   parts[1],
		OnDelete: OnDeleteRestrict,
	}

	for _, param := range parts[2:] {
		pKey, pValue, pHasValue := ParseKeyValue(param)
		if pKey != "on-delete" || !pHasValue {
			return fmt.Errorf("unknown relation parameter %q, expected 'on-delete=<action>'", param)
		}

		switch OnDeleteAction(pValue) {
		case OnDeleteRestrict, OnDeleteCascade, OnDeleteSetNull:
			rel.OnDelete = OnDeleteAction(pValue)
		default:
			return fmt.Errorf("unknown on-delete action %q, expected 'restrict', 'cascade', or 'set-null'", pValue)
		}
	}

	result.Relation = rel
	return nil
}

// parseDefaultAnnotation parses +fabrica:field:default=<value>
//
// Example:
//   - +fabrica:field:default=true
//   - +fabrica:field:default=0
func parseDefaultAnnotation(result *FieldAnnotations, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("default annotation missing")
	}

	key, value, hasValue := ParseKeyValue(parts[0])
	if !hasValue || key != "default" {
		return fmt.Errorf("expected format: default=<value>")
	}

	result.Default = value
	return nil
}

// ParseResourceFile returns the annotations for a single resource type in a
// file, merging the field annotations declared on its <Name>Spec companion.
//
// Fabrica splits a resource across two declarations: type-level annotations sit
// on <Name>, field-level annotations on <Name>Spec. Anything that wants a
// complete, validatable picture of a resource needs both, so this does the
// merge in one place.
//
// Declaration order does not matter: <Name>Spec may appear before or after
// <Name>. A resource that does not appear in the file yields empty annotations
// rather than an error, matching the previous behaviour of callers.
func ParseResourceFile(filename, resourceName string) (*ResourceAnnotations, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file %s: %w", filename, err)
	}

	return parseResourceFromFile(file, resourceName)
}

// parseResourceFromFile does the two-declaration merge over an already-parsed
// file. Kept separate so callers that already hold an *ast.File do not reparse.
func parseResourceFromFile(file *ast.File, resourceName string) (*ResourceAnnotations, error) {
	specTypeName := resourceName + "Spec"

	var (
		resourceAnnots *ResourceAnnotations
		specFields     map[string]*FieldAnnotations
	)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			switch typeSpec.Name.Name {
			case resourceName:
				annots, err := ParseResourceAnnotations(typeSpec, genDecl.Doc)
				if err != nil {
					return nil, fmt.Errorf("parse annotations on %s: %w", resourceName, err)
				}
				resourceAnnots = annots

			case specTypeName:
				annots, err := ParseResourceAnnotations(typeSpec, genDecl.Doc)
				if err != nil {
					return nil, fmt.Errorf("parse annotations on %s: %w", specTypeName, err)
				}
				if specFields == nil {
					specFields = make(map[string]*FieldAnnotations)
				}
				for fieldName, fieldAnnots := range annots.Fields {
					specFields[fieldName] = fieldAnnots
				}
			}
		}
	}

	// Merge after the whole file is walked, so <Name>Spec declared before
	// <Name> is not discarded when the resource declaration is reached.
	if resourceAnnots == nil {
		if specFields == nil {
			return NewResourceAnnotations(), nil
		}
		resourceAnnots = NewResourceAnnotations()
	}

	for fieldName, fieldAnnots := range specFields {
		resourceAnnots.Fields[fieldName] = fieldAnnots
	}

	return resourceAnnots, nil
}

// mergeSpecFields folds each <Name>Spec's field annotations into <Name> so that
// every resource entry in the map is complete and validatable on its own. The
// <Name>Spec entries are left in place for backward compatibility.
func mergeSpecFields(byType map[string]*ResourceAnnotations) {
	for typeName, annots := range byType {
		spec, ok := byType[typeName+"Spec"]
		if !ok {
			continue
		}
		for fieldName, fieldAnnots := range spec.Fields {
			annots.Fields[fieldName] = fieldAnnots
		}
	}
}

// ParseFileAnnotations parses annotations from a Go source file with caching
//
// This function parses a Go source file and extracts Fabrica annotations.
// Results are cached based on file modification time for performance.
//
// Example:
//
//	annotations, err := ParseFileAnnotations("apis/v1/user_types.go")
//	if err != nil {
//	    return err
//	}
//	for fieldName, ann := range annotations {
//	    fmt.Printf("%s: %+v\n", fieldName, ann)
//	}
func ParseFileAnnotations(filename string) (map[string]*ResourceAnnotations, error) {
	if cached, ok := globalCache.Get(filename); ok {
		return cached, nil
	}

	// Parse file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	result := make(map[string]*ResourceAnnotations)

	// Walk declarations
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			annotations, err := ParseResourceAnnotations(typeSpec, genDecl.Doc)
			if err != nil {
				return nil, err
			}

			result[typeSpec.Name.Name] = annotations
		}
	}

	mergeSpecFields(result)
	globalCache.Set(filename, result)

	return result, nil
}

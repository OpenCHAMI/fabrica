// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
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

	default:
		// Try to parse as key=value format (e.g., storage=dedicated)
		key, value, hasValue := ParseKeyValue(parts[0])
		if !hasValue {
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

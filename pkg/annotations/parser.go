// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"strings"
)

// parseResourceLevelAnnotation processes a single resource-level annotation
func parseResourceLevelAnnotation(
	result *ResourceAnnotations,
	annotation string,
	seen map[string]string,
) error {
	parts, err := strictAnnotationParts(annotation)
	if err != nil {
		return parseError(parseSource{directive: annotation}, err.Error(), err)
	}
	key := directiveKey(parts[0])
	if err := recordDirective(seen, key, annotation); err != nil {
		return parseError(parseSource{directive: annotation}, err.Error(), err)
	}

	switch key {
	case "resource":
		if len(parts) != 1 || parts[0] != "resource" {
			return parseError(parseSource{directive: annotation}, "resource directive does not accept a value", nil)
		}
		result.IsResource = true
		return nil
	case "storage":
		if len(parts) != 1 {
			return parseError(parseSource{directive: annotation}, "resource storage directive has trailing parameters", nil)
		}
		key, value, hasValue := ParseKeyValue(parts[0])
		if !hasValue || key != "storage" || value == "" {
			return parseError(parseSource{directive: annotation}, "expected format: storage=<mode>", nil)
		}
		switch StorageMode(value) {
		case StorageModeGeneric:
			result.StorageMode = StorageModeGeneric
		case StorageModeDedicated:
			result.StorageMode = StorageModeDedicated
		default:
			message := fmt.Sprintf("unknown storage mode %q, expected 'generic' or 'dedicated'", value)
			return parseError(parseSource{directive: annotation}, message, nil)
		}
		return nil
	case "verbs":
		if len(parts) != 1 {
			return parseError(parseSource{directive: annotation}, "resource verbs directive has trailing parameters", nil)
		}
		name, value, hasValue := ParseKeyValue(parts[0])
		if !hasValue || name != "verbs" || value == "" {
			return parseError(parseSource{directive: annotation}, "expected format: verbs=<csv>", nil)
		}
		verbs, parseErr := parseOperationVerbs(value)
		if parseErr != nil {
			return parseError(parseSource{directive: annotation}, parseErr.Error(), parseErr)
		}
		result.Verbs = verbs
		result.VerbsExplicit = true
		return nil
	case "exposure":
		if len(parts) != 1 {
			return parseError(parseSource{directive: annotation}, "resource exposure directive has trailing parameters", nil)
		}
		name, value, hasValue := ParseKeyValue(parts[0])
		if !hasValue || name != "exposure" || value == "" {
			return parseError(parseSource{directive: annotation}, "expected format: exposure=<value>", nil)
		}
		exposure := Exposure(value)
		switch exposure {
		case ExposureDefault, ExposurePublic, ExposureProtected, ExposureInternal, ExposurePrivate:
			result.Exposure = exposure
			return nil
		default:
			return parseError(parseSource{directive: annotation}, fmt.Sprintf("unknown exposure %q", value), nil)
		}
	default:
		return unknownDirectiveError("resource", key, resourceDirectiveKeys, annotation)
	}
}

func parseOperationVerbs(value string) ([]OperationVerb, error) {
	members := strings.Split(value, ",")
	verbs := make([]OperationVerb, 0, len(members))
	seen := make(map[OperationVerb]struct{}, len(members))
	for _, member := range members {
		if member == "" || strings.TrimSpace(member) == "" {
			return nil, fmt.Errorf("empty operation verb in verbs list")
		}
		if member != strings.TrimSpace(member) {
			return nil, fmt.Errorf("operation verb %q contains whitespace", member)
		}
		verb := OperationVerb(member)
		switch verb {
		case OperationList, OperationGet, OperationCreate, OperationUpdate, OperationPatch, OperationDelete,
			OperationStatusUpdate, OperationStatusPatch, OperationVersionList, OperationVersionGet,
			OperationVersionDelete, OperationAll, OperationNone:
		default:
			return nil, fmt.Errorf("unknown operation verb %q", member)
		}
		if _, duplicate := seen[verb]; duplicate {
			return nil, fmt.Errorf("duplicate operation verb %q", member)
		}
		seen[verb] = struct{}{}
		verbs = append(verbs, verb)
	}
	return verbs, nil
}

// parseFieldLevelAnnotation processes a single field-level annotation
func parseFieldLevelAnnotation(
	result *FieldAnnotations,
	annotation string,
	seen map[string]string,
) error {
	parts, err := strictAnnotationParts(annotation)
	if err != nil {
		return parseError(parseSource{directive: annotation}, err.Error(), err)
	}

	// All field annotations start with "field:"
	if parts[0] != "field" {
		return parseError(parseSource{directive: annotation}, "field annotations must start with +fabrica:field:", nil)
	}

	if len(parts) < 2 {
		return parseError(parseSource{directive: annotation}, "field annotation requires a directive after 'field:'", nil)
	}

	directive := parts[1]
	key, _, hasValue := ParseKeyValue(directive)
	if err := recordDirective(seen, key, annotation); err != nil {
		return parseError(parseSource{directive: annotation}, err.Error(), err)
	}

	switch key {
	case "sensitive":
		if hasValue || len(parts) != 2 {
			return parseError(parseSource{directive: annotation}, "sensitive directive does not accept parameters", nil)
		}
		result.Sensitive = true
		return nil

	case "immutable":
		if hasValue || len(parts) != 2 {
			return parseError(parseSource{directive: annotation}, "immutable directive does not accept parameters", nil)
		}
		result.Immutable = true
		return nil

	case "unique":
		if hasValue || len(parts) != 2 {
			return parseError(parseSource{directive: annotation}, "unique directive does not accept parameters", nil)
		}
		result.Unique = true
		return nil

	case "storage":
		return parseStorageAnnotation(result, parts[1:], annotation)

	case "index":
		if len(parts) != 2 {
			return parseError(parseSource{directive: annotation}, "index directive has trailing parameters", nil)
		}
		return parseIndexAnnotation(result, parts[1:], annotation)

	case "default":
		if len(parts) != 2 {
			return parseError(parseSource{directive: annotation}, "default directive has trailing parameters", nil)
		}
		return parseDefaultAnnotation(result, parts[1:], annotation)

	default:
		return unknownDirectiveError("field", key, fieldDirectiveKeys, annotation)
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
		value, present, err := parseSingleStorageParameter(parts[1:], "cost")
		if err != nil {
			return err
		}
		if present {
			cost, err := ParseIntValue(value, 4, 31)
			if err != nil {
				return fmt.Errorf("bcrypt cost: %w", err)
			}
			config.Hash.Cost = cost
		}
		return nil

	case HashAlgorithmArgon2:
		// Default argon2 parameters
		config.Hash.Cost = 65536 // 64MB memory

		value, present, err := parseSingleStorageParameter(parts[1:], "memory")
		if err != nil {
			return err
		}
		if present {
			memory, err := ParseIntValue(value, 1024, 1048576)
			if err != nil {
				return fmt.Errorf("argon2 memory: %w", err)
			}
			config.Hash.Cost = memory
		}
		return nil

	case HashAlgorithmSHA256:
		_, _, err := parseSingleStorageParameter(parts[1:], "")
		return err

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

	value, present, err := parseSingleStorageParameter(parts[1:], "key")
	if err != nil {
		return err
	}
	if present {
		config.Encryption.KeySource = value
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

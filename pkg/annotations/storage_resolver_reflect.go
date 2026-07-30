// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import "reflect"

// ReflectedFieldStorage supplies field identity and presence facts for programmatic registration.
type ReflectedFieldStorage struct {
	GoName   string
	JSONName string
	GoType   reflect.Type
	Required bool
}

// ResolveStorageIntentFromReflect validates programmatic registration against the same contract as source parsing.
func ResolveStorageIntentFromReflect(
	resourceName string,
	fields []ReflectedFieldStorage,
	raw *ResourceAnnotations,
	dialect Dialect,
) (*ResolvedResourceStorage, error) {
	source := SourcePosition{TypeName: resourceName, Directive: "programmatic registration"}
	if raw == nil {
		return nil, capabilityError(source, CapabilityUnknown, "resource annotations are missing")
	}
	if dialect != DialectPostgreSQL && dialect != DialectSQLite {
		return nil, capabilityError(source, CapabilityDialect, "database dialect is unknown")
	}
	resolved := &ResolvedResourceStorage{
		Source: source, Name: resourceName, Storage: resolvedStorageKind(raw.StorageMode), Dialect: dialect,
		Fields: make([]ResolvedFieldStorage, 0, len(fields)),
	}
	for _, field := range fields {
		intent, err := resolveReflectedField(field, raw.Fields[field.GoName], dialect, resourceName)
		if err != nil {
			return nil, err
		}
		resolved.Fields = append(resolved.Fields, intent)
	}
	return resolved, nil
}

func resolveReflectedField(
	field ReflectedFieldStorage,
	raw *FieldAnnotations,
	dialect Dialect,
	resourceName string,
) (ResolvedFieldStorage, error) {
	source := SourcePosition{
		TypeName: resourceName + "Spec", FieldName: field.GoName, Directive: "programmatic registration",
	}
	if field.GoType == nil {
		return ResolvedFieldStorage{}, capabilityError(source, CapabilityFieldType, "reflected Go type is missing")
	}
	fieldType, err := FieldTypeFromReflect(field.GoType, source)
	if err != nil {
		return ResolvedFieldStorage{}, err
	}
	optionality := OptionalityOptional
	if fieldType.Pointer() {
		optionality = OptionalityNillable
	} else if field.Required {
		optionality = OptionalityRequired
	}
	intent := ResolvedFieldStorage{
		Source: source, GoName: field.GoName, JSONName: field.JSONName, Type: fieldType,
		Optionality: optionality, Transform: StorageTransform{Kind: TransformStandard},
		Index: IndexNone, Dialect: dialect,
	}
	if raw == nil {
		return intent, nil
	}
	intent.Sensitive = raw.Sensitive
	intent.Immutable = raw.Immutable
	intent.Unique = raw.Unique
	if raw.Storage != nil {
		transform, transformErr := resolveTransform(fieldType, raw.Storage, source)
		if transformErr != nil {
			return ResolvedFieldStorage{}, transformErr
		}
		intent.Transform = transform
	}
	if raw.Index != nil {
		index, indexErr := (storageResolver{dialect: dialect}).resolveIndex(fieldType, raw.Index, source)
		if indexErr != nil {
			return ResolvedFieldStorage{}, indexErr
		}
		intent.Index = index
	}
	if hasRawDefault(raw) {
		if intent.Immutable {
			return ResolvedFieldStorage{}, defaultError(source, fieldType.Kind, DefaultErrorConflict, "immutable fields cannot have database defaults", nil)
		}
		if intent.Transform.Kind != TransformStandard {
			return ResolvedFieldStorage{}, defaultError(source, fieldType.Kind, DefaultErrorConflict, "transformed fields cannot have database defaults", nil)
		}
		value, defaultErr := parseDefaultValue(fieldType, raw.Default, source)
		if defaultErr != nil {
			return ResolvedFieldStorage{}, defaultErr
		}
		intent.Default = value
	}
	return intent, nil
}

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import "slices"

func cloneResourceAnnotationsMap(source map[string]*ResourceAnnotations) map[string]*ResourceAnnotations {
	if source == nil {
		return nil
	}
	cloned := make(map[string]*ResourceAnnotations, len(source))
	for name, annotations := range source {
		cloned[name] = cloneResourceAnnotations(annotations)
	}
	return cloned
}

func cloneResourceAnnotations(source *ResourceAnnotations) *ResourceAnnotations {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Fields = cloneFieldAnnotationsMap(source.Fields)
	cloned.RawAnnotations = slices.Clone(source.RawAnnotations)
	return &cloned
}

func cloneFieldAnnotationsMap(source map[string]*FieldAnnotations) map[string]*FieldAnnotations {
	if source == nil {
		return nil
	}
	cloned := make(map[string]*FieldAnnotations, len(source))
	for name, annotations := range source {
		cloned[name] = cloneFieldAnnotations(annotations)
	}
	return cloned
}

func cloneFieldAnnotations(source *FieldAnnotations) *FieldAnnotations {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.RawAnnotations = slices.Clone(source.RawAnnotations)
	if source.Storage != nil {
		storage := *source.Storage
		if source.Storage.Hash != nil {
			hash := *source.Storage.Hash
			storage.Hash = &hash
		}
		if source.Storage.Encryption != nil {
			encryption := *source.Storage.Encryption
			storage.Encryption = &encryption
		}
		cloned.Storage = &storage
	}
	if source.Index != nil {
		index := *source.Index
		cloned.Index = &index
	}
	return &cloned
}

func cloneResolvedResourceStorage(source *ResolvedResourceStorage) *ResolvedResourceStorage {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Fields = slices.Clone(source.Fields)
	for index := range cloned.Fields {
		cloned.Fields[index].Default = cloneDefaultValue(source.Fields[index].Default)
	}
	return &cloned
}

func cloneDefaultValue(source DefaultValue) DefaultValue {
	switch value := source.(type) {
	case nil:
		return nil
	case StringDefault:
		return StringDefault{Value: value.Value}
	case BoolDefault:
		return BoolDefault{Value: value.Value}
	case IntDefault:
		return IntDefault{Value: value.Value}
	case Int64Default:
		return Int64Default{Value: value.Value}
	case Float64Default:
		return Float64Default{Value: value.Value}
	default:
		return value
	}
}

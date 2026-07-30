// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"reflect"
	"testing"
	"time"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestFieldType_reflection_bridge_preserves_concrete_spec_types(t *testing.T) {
	// Given
	type RecordSpec struct {
		Name      string      `json:"name" validate:"required"`
		Count     *int        `json:"count,omitempty"`
		CreatedAt time.Time   `json:"createdAt"`
		Tags      []string    `json:"tags,omitempty"`
		Ignored   map[int]int `json:"ignored"`
	}
	type Record struct {
		Spec RecordSpec
	}

	// When
	fields := extractSpecFields(reflect.TypeOf(Record{}))

	// Then
	if len(fields) != 5 {
		t.Fatalf("extractSpecFields() count = %d", len(fields))
	}
	want := []reflect.Type{
		reflect.TypeOf(""), reflect.TypeOf((*int)(nil)), reflect.TypeOf(time.Time{}),
		reflect.TypeOf([]string{}), reflect.TypeOf(map[int]int{}),
	}
	for index, field := range fields {
		if field.GoType != want[index] {
			t.Errorf("field %s GoType = %v, want %v", field.Name, field.GoType, want[index])
		}
	}
	resolved, err := annotations.FieldTypeFromReflect(fields[1].GoType, annotations.SourcePosition{Directive: "field declaration"})
	if err != nil || resolved.Kind != annotations.FieldKindInt || !resolved.Pointer() {
		t.Errorf("FieldTypeFromReflect(pointer) = %#v, %v", resolved, err)
	}
	if _, err := annotations.FieldTypeFromReflect(fields[4].GoType, annotations.SourcePosition{Directive: "field declaration"}); err == nil {
		t.Error("FieldTypeFromReflect(map) error = nil")
	}
}

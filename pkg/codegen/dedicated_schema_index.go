// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/openchami/fabrica/pkg/annotations"
)

type dedicatedSchemaIndex struct {
	ColumnName string
	Method     string
}

func buildDedicatedSchemaIndex(
	field annotations.ResolvedFieldStorage,
	columnName string,
	dialect annotations.Dialect,
) (*dedicatedSchemaIndex, error) {
	if field.Dialect != dialect {
		return nil, fmt.Errorf("field dialect does not match resource dialect")
	}
	switch field.Index {
	case annotations.IndexNone:
		return nil, nil
	case annotations.IndexBTree:
		if field.Unique {
			return nil, nil
		}
		return &dedicatedSchemaIndex{ColumnName: columnName}, nil
	case annotations.IndexGIN:
		if dialect != annotations.DialectPostgreSQL || field.Type.Kind != annotations.FieldKindStringSlice {
			return nil, fmt.Errorf("GIN index is unsupported for the resolved dialect or field type")
		}
		return &dedicatedSchemaIndex{ColumnName: columnName, Method: "GIN"}, nil
	case annotations.IndexHash:
		if dialect != annotations.DialectPostgreSQL || !dedicatedHashIndexKind(field.Type.Kind) {
			return nil, fmt.Errorf("hash index is unsupported for the resolved dialect or field type")
		}
		return &dedicatedSchemaIndex{ColumnName: columnName, Method: "HASH"}, nil
	case annotations.IndexGiST:
		return nil, fmt.Errorf("GiST index is unsupported for the current capability matrix")
	case annotations.IndexUnknown:
		return nil, fmt.Errorf("index kind is unknown")
	default:
		return nil, fmt.Errorf("index kind is unsupported")
	}
}

func dedicatedHashIndexKind(kind annotations.FieldKind) bool {
	switch kind {
	case annotations.FieldKindString,
		annotations.FieldKindBool,
		annotations.FieldKindInt,
		annotations.FieldKindInt64,
		annotations.FieldKindFloat64,
		annotations.FieldKindTime:
		return true
	case annotations.FieldKindStringSlice, annotations.FieldKindUnknown:
		return false
	default:
		return false
	}
}

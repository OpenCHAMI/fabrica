// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"strconv"
	"strings"
)

// DefaultValue is the closed set of scalar defaults supported by dedicated storage.
type DefaultValue interface {
	isDefaultValue()
}

// StringDefault holds a present string default, including an empty string.
type StringDefault struct{ Value string }

// BoolDefault holds a present boolean default, including false.
type BoolDefault struct{ Value bool }

// IntDefault holds a present platform-sized integer default, including zero.
type IntDefault struct{ Value int }

// Int64Default holds a present 64-bit integer default, including zero.
type Int64Default struct{ Value int64 }

// Float64Default holds a present finite floating-point default, including zero.
type Float64Default struct{ Value float64 }

func (StringDefault) isDefaultValue()  {}
func (BoolDefault) isDefaultValue()    {}
func (IntDefault) isDefaultValue()     {}
func (Int64Default) isDefaultValue()   {}
func (Float64Default) isDefaultValue() {}

func parseDefaultValue(fieldType FieldType, raw string, source SourcePosition) (DefaultValue, error) {
	switch fieldType.Kind {
	case FieldKindString:
		return StringDefault{Value: raw}, nil
	case FieldKindBool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, defaultError(source, fieldType.Kind, DefaultErrorInvalidLiteral, "value must be a boolean literal", err)
		}
		return BoolDefault{Value: value}, nil
	case FieldKindInt:
		value, err := strconv.ParseInt(raw, 10, strconv.IntSize)
		if err != nil {
			return nil, defaultError(source, fieldType.Kind, DefaultErrorInvalidLiteral, "value must fit the Go int type", err)
		}
		return IntDefault{Value: int(value)}, nil
	case FieldKindInt64:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, defaultError(source, fieldType.Kind, DefaultErrorInvalidLiteral, "value must fit the Go int64 type", err)
		}
		return Int64Default{Value: value}, nil
	case FieldKindFloat64:
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, defaultError(source, fieldType.Kind, DefaultErrorInvalidLiteral, "value must be a float64 literal", err)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, defaultError(source, fieldType.Kind, DefaultErrorInvalidLiteral, "NaN and infinity are not supported", nil)
		}
		return Float64Default{Value: value}, nil
	case FieldKindTime, FieldKindStringSlice, FieldKindUnknown:
		return nil, defaultError(source, fieldType.Kind, DefaultErrorUnsupportedType, "field type does not support defaults", nil)
	default:
		return nil, defaultError(source, fieldType.Kind, DefaultErrorUnsupportedType, "field type does not support defaults", nil)
	}
}

// GoLiteral renders a parsed default as a safe Go expression.
func GoLiteral(value DefaultValue) (string, error) {
	switch typed := value.(type) {
	case StringDefault:
		return strconv.Quote(typed.Value), nil
	case BoolDefault:
		return strconv.FormatBool(typed.Value), nil
	case IntDefault:
		return strconv.FormatInt(int64(typed.Value), 10), nil
	case Int64Default:
		return strconv.FormatInt(typed.Value, 10), nil
	case Float64Default:
		literal := strconv.FormatFloat(typed.Value, 'g', -1, 64)
		if !strings.ContainsAny(literal, ".eE") {
			literal += ".0"
		}
		return literal, nil
	default:
		return "", defaultError(SourcePosition{}, FieldKindUnknown, DefaultErrorUnsupportedType, fmt.Sprintf("unsupported default variant %T", value), nil)
	}
}

func hasRawDefault(raw *FieldAnnotations) bool {
	if raw == nil {
		return false
	}
	if raw.Default != "" {
		return true
	}
	for _, annotation := range raw.RawAnnotations {
		if strings.HasPrefix(annotation, "+fabrica:field:default=") {
			return true
		}
	}
	return false
}

func defaultDirectiveSource(
	field *ast.Field,
	fset *token.FileSet,
	typeName string,
	fieldName string,
	defaultValue string,
) SourcePosition {
	source := sourceAt(fset.Position(field.Pos()), SourcePosition{
		TypeName: typeName, FieldName: fieldName, Directive: "+fabrica:field:default=" + defaultValue,
	})
	if field.Doc == nil {
		return source
	}
	for _, comment := range field.Doc.List {
		directive := CleanAnnotationLine(comment.Text)
		if strings.HasPrefix(directive, "+fabrica:field:default=") {
			return sourceAt(fset.Position(comment.Slash), SourcePosition{
				TypeName: typeName, FieldName: fieldName, Directive: directive,
			})
		}
	}
	return source
}

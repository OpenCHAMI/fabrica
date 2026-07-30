// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
	"reflect"
	"time"
)

var (
	stringType      = reflect.TypeOf("")
	boolType        = reflect.TypeOf(false)
	intType         = reflect.TypeOf(int(0))
	int64Type       = reflect.TypeOf(int64(0))
	float64Type     = reflect.TypeOf(float64(0))
	timeType        = reflect.TypeOf(time.Time{})
	stringSliceType = reflect.TypeOf([]string{})
)

// FieldTypeFromReflect resolves a reflected Go type against the closed dedicated-storage type set.
func FieldTypeFromReflect(goType reflect.Type, source SourcePosition) (FieldType, error) {
	pointer := goType.Kind() == reflect.Pointer
	base := goType
	if pointer {
		base = goType.Elem()
	}

	kind := fieldKind(base)
	if kind == FieldKindUnknown || pointer && kind == FieldKindStringSlice {
		return FieldType{}, capabilityError(source, CapabilityFieldType, fmt.Sprintf("Go type %s is not supported by dedicated Ent storage", goType))
	}
	return FieldType{Kind: kind, pointer: pointer, goType: goType}, nil
}

func fieldKind(goType reflect.Type) FieldKind {
	switch goType {
	case stringType:
		return FieldKindString
	case boolType:
		return FieldKindBool
	case intType:
		return FieldKindInt
	case int64Type:
		return FieldKindInt64
	case float64Type:
		return FieldKindFloat64
	case timeType:
		return FieldKindTime
	case stringSliceType:
		return FieldKindStringSlice
	default:
		return FieldKindUnknown
	}
}

func fieldTypeFromAST(expression ast.Expr, source SourcePosition) (FieldType, error) {
	goType := reflect.Type(nil)
	switch value := expression.(type) {
	case *ast.Ident:
		goType = scalarType(value.Name)
	case *ast.SelectorExpr:
		packageName, ok := value.X.(*ast.Ident)
		if ok && packageName.Name == "time" && value.Sel.Name == "Time" {
			goType = timeType
		}
	case *ast.ArrayType:
		if value.Len == nil {
			if element, ok := value.Elt.(*ast.Ident); ok && element.Name == "string" {
				goType = stringSliceType
			}
		}
	case *ast.StarExpr:
		base, err := fieldTypeFromAST(value.X, source)
		if err != nil {
			return FieldType{}, err
		}
		return FieldTypeFromReflect(reflect.PointerTo(base.GoType()), source)
	}
	if goType == nil {
		return FieldType{}, capabilityError(source, CapabilityFieldType, "field Go type is not supported by dedicated Ent storage")
	}
	return FieldTypeFromReflect(goType, source)
}

func scalarType(name string) reflect.Type {
	switch name {
	case "string":
		return stringType
	case "bool":
		return boolType
	case "int":
		return intType
	case "int64":
		return int64Type
	case "float64":
		return float64Type
	default:
		return nil
	}
}

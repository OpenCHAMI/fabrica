// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"fmt"
)

// ErrInvalidDefault identifies invalid or conflicting default directives.
var ErrInvalidDefault = errors.New("invalid annotation default")

// DefaultErrorKind classifies failures while resolving a default directive.
type DefaultErrorKind uint8

const (
	// DefaultErrorUnknown reports a default failure that has no narrower classification.
	DefaultErrorUnknown DefaultErrorKind = iota
	// DefaultErrorInvalidLiteral reports a value that cannot be parsed for its field type.
	DefaultErrorInvalidLiteral
	// DefaultErrorUnsupportedType reports a field kind that cannot have database defaults.
	DefaultErrorUnsupportedType
	// DefaultErrorConflict reports a default combined with an incompatible directive.
	DefaultErrorConflict
)

// DefaultError carries source and field context for a rejected default.
type DefaultError struct {
	Filename  string
	Line      int
	Column    int
	TypeName  string
	FieldName string
	Directive string
	FieldKind FieldKind
	Kind      DefaultErrorKind
	Message   string
	Cause     error
}

func (e *DefaultError) Error() string {
	location := e.Filename
	if e.Line > 0 {
		location = fmt.Sprintf("%s:%d:%d", location, e.Line, e.Column)
	}
	declaration := e.TypeName
	if e.FieldName != "" {
		declaration += "." + e.FieldName
	}
	return fmt.Sprintf("%s: %s: default %q: %s", location, declaration, e.Directive, e.Message)
}

// Is reports whether target identifies the invalid-default error family.
func (e *DefaultError) Is(target error) bool {
	return target == ErrInvalidDefault
}

func (e *DefaultError) Unwrap() error {
	return e.Cause
}

func defaultError(source SourcePosition, fieldKind FieldKind, kind DefaultErrorKind, message string, cause error) *DefaultError {
	return &DefaultError{
		Filename: source.Filename, Line: source.Line, Column: source.Column,
		TypeName: source.TypeName, FieldName: source.FieldName, Directive: source.Directive,
		FieldKind: fieldKind, Kind: kind, Message: message, Cause: cause,
	}
}

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"fmt"
	"go/token"
)

// ErrAnnotationParse identifies malformed or unknown Fabrica annotations.
var ErrAnnotationParse = errors.New("annotation parse failure")

// ParseError records source context and safe remediation for a rejected annotation.
type ParseError struct {
	Filename   string
	Line       int
	Column     int
	TypeName   string
	FieldName  string
	Directive  string
	Suggestion string
	Message    string
	Cause      error
}

func (e *ParseError) Error() string {
	location := e.Filename
	if e.Line > 0 {
		location = fmt.Sprintf("%s:%d:%d", location, e.Line, e.Column)
	}
	context := e.TypeName
	if e.FieldName != "" {
		context += "." + e.FieldName
	}
	if location != "" {
		return fmt.Sprintf("%s: %s: annotation %q: %s", location, context, e.Directive, e.Message)
	}
	return fmt.Sprintf("%s: annotation %q: %s", context, e.Directive, e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// Is reports whether target identifies the annotation-parse error family.
func (e *ParseError) Is(target error) bool {
	return target == ErrAnnotationParse
}

type parseSource struct {
	position  token.Position
	typeName  string
	fieldName string
	directive string
}

func parseError(source parseSource, message string, cause error) *ParseError {
	return &ParseError{
		Filename:  source.position.Filename,
		Line:      source.position.Line,
		Column:    source.position.Column,
		TypeName:  source.typeName,
		FieldName: source.fieldName,
		Directive: source.directive,
		Message:   message,
		Cause:     cause,
	}
}

func contextualizeParseError(err error, source parseSource) *ParseError {
	var existing *ParseError
	if !errors.As(err, &existing) {
		return parseError(source, err.Error(), err)
	}
	if source.directive == "" {
		source.directive = existing.Directive
	}
	if source.fieldName == "" {
		source.fieldName = existing.FieldName
	}
	result := parseError(source, existing.Message, existing.Cause)
	result.Suggestion = existing.Suggestion
	return result
}

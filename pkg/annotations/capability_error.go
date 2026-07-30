// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"fmt"
)

// ErrUnsupportedCapability identifies a directive that cannot be implemented by the selected storage contract.
var ErrUnsupportedCapability = errors.New("unsupported annotation storage capability")

// CapabilityKind classifies the unsupported part of a storage request.
type CapabilityKind uint8

const (
	// CapabilityUnknown reports an unsupported request that has no narrower classification.
	CapabilityUnknown CapabilityKind = iota
	// CapabilityDialect reports an unsupported database dialect.
	CapabilityDialect
	// CapabilityFieldType reports a Go type outside the dedicated-storage field set.
	CapabilityFieldType
	// CapabilityTransform reports an unsupported or incompatible storage transform.
	CapabilityTransform
	// CapabilityIndex reports an unsupported index method or field/index combination.
	CapabilityIndex
	// CapabilityBackend reports a storage mode or directive unsupported by the selected backend.
	CapabilityBackend
)

// CapabilityError records the source and semantic context of an unsupported storage request.
type CapabilityError struct {
	Filename   string
	Line       int
	Column     int
	TypeName   string
	FieldName  string
	Directive  string
	Capability CapabilityKind
	Message    string
	Cause      error
}

func (e *CapabilityError) Error() string {
	location := e.Filename
	if e.Line > 0 {
		location = fmt.Sprintf("%s:%d:%d", location, e.Line, e.Column)
	}
	declaration := e.TypeName
	if e.FieldName != "" {
		declaration += "." + e.FieldName
	}
	return fmt.Sprintf("%s: %s: capability %q: %s", location, declaration, e.Directive, e.Message)
}

// Is reports whether target identifies the unsupported-capability error family.
func (e *CapabilityError) Is(target error) bool {
	return target == ErrUnsupportedCapability
}

func (e *CapabilityError) Unwrap() error {
	return e.Cause
}

func capabilityError(source SourcePosition, capability CapabilityKind, message string) *CapabilityError {
	return &CapabilityError{
		Filename: source.Filename, Line: source.Line, Column: source.Column,
		TypeName: source.TypeName, FieldName: source.FieldName, Directive: source.Directive,
		Capability: capability, Message: message,
	}
}

// ParseDialect maps a configured driver name to a supported annotation storage dialect.
func ParseDialect(driver string, source SourcePosition) (Dialect, error) {
	switch driver {
	case "postgres", "postgresql":
		return DialectPostgreSQL, nil
	case "sqlite", "sqlite3":
		return DialectSQLite, nil
	default:
		return DialectUnknown, capabilityError(source, CapabilityDialect, fmt.Sprintf("database dialect %q is not supported", driver))
	}
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package validation

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Mode controls optional Spec/Status tags and application validation hooks.
// It never controls decoding, authorization, preconditions, or storage constraints.
type Mode string

const (
	// Strict rejects optional validation failures.
	Strict Mode = "strict"
	// Warn logs optional validation failures and continues.
	Warn Mode = "warn"
	// Disabled skips optional validators, including custom hooks.
	Disabled Mode = "disabled"
)

// ResolveMode validates the configured mode before applying the enabled flag.
// An omitted mode defaults to strict; enabled=false always disables optional checks.
func ResolveMode(enabled bool, mode string) (Mode, error) {
	switch Mode(mode) {
	case "", Strict, Warn, Disabled:
	default:
		return "", fmt.Errorf("invalid validation.mode: %s (must be 'strict', 'warn', or 'disabled')", mode)
	}
	if !enabled || Mode(mode) == Disabled {
		return Disabled, nil
	}
	if mode == "" {
		return Strict, nil
	}
	return Mode(mode), nil
}

// ValidateEnvelope checks tags outside the contents of the top-level Spec and
// Status fields. Tags on those fields themselves (such as required on a pointer)
// still apply. Generated handlers use this for mandatory request requirements.
func ValidateEnvelope(value any) error {
	err := validate.StructFiltered(value, func(namespace []byte) bool {
		_, field, ok := strings.Cut(string(namespace), ".")
		if !ok {
			return false
		}
		return strings.HasPrefix(field, "Spec.") || strings.HasPrefix(field, "Spec[") ||
			strings.HasPrefix(field, "Status.") || strings.HasPrefix(field, "Status[")
	})
	if errs, ok := err.(validator.ValidationErrors); ok {
		return formatValidationErrors(errs)
	}
	return err
}

// ValidateRequest preserves envelope checks while applying mode to Spec/Status tags.
func ValidateRequest(value any, mode Mode) error {
	if err := ValidateEnvelope(value); err != nil {
		return err
	}
	return optionalResult(mode, value, func() error { return ValidateResource(value) })
}

// ValidateRequestWithContext additionally invokes application validation hooks.
// Like atomic storage callbacks, these hooks must be safe to retry. Warn logs may
// repeat when a mutation is retried.
func ValidateRequestWithContext(ctx context.Context, value any, mode Mode) error {
	if err := ValidateEnvelope(value); err != nil {
		return err
	}
	return ValidateWithContextMode(ctx, value, mode)
}

// ValidateWithContextMode applies mode to all tags and custom hooks on value.
// Use this for a standalone Spec or Status; use ValidateRequestWithContext for
// resource envelopes. The original ValidateWithContext remains strictly validating.
func ValidateWithContextMode(ctx context.Context, value any, mode Mode) error {
	if err := optionalResult(mode, value, func() error { return ValidateResource(value) }); err != nil {
		return err
	}
	return optionalResult(mode, value, func() error {
		if custom, ok := value.(CustomValidator); ok {
			return custom.Validate(ctx)
		}
		return nil
	})
}

func optionalResult(mode Mode, value any, check func() error) error {
	switch mode {
	case Disabled:
		return nil
	case Strict, Warn:
		err := check()
		if err != nil && mode == Warn {
			log.Printf("WARN: validation failed for %T: %v", value, err)
			return nil
		}
		return err
	default:
		// Fail closed for callers that bypass ResolveMode.
		return fmt.Errorf("invalid effective validation mode %q", mode)
	}
}

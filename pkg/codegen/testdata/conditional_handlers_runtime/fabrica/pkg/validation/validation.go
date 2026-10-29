// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package validation

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
)

var Reject atomic.Bool

// Hook exercises application validation against the generated storage layer.
var Hook func(context.Context, any) error

type requiredFieldPresence interface {
	RequiredFieldPresent(name string) (bool, bool)
}

func ValidateResource(any) error {
	return nil
}

func ValidateRequiredFields(value any, selected ...string) error {
	presence, tracksPresence := value.(requiredFieldPresence)
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer {
		reflected = reflected.Elem()
	}
	typeOfValue := reflected.Type()
	for index := 0; index < reflected.NumField(); index++ {
		field := typeOfValue.Field(index)
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		selectedField := len(selected) == 0 || slices.Contains(selected, name)
		missing := reflected.Field(index).IsZero()
		if tracksPresence {
			if present, tracked := presence.RequiredFieldPresent(name); tracked {
				missing = !present
			}
		}
		if selectedField && strings.Contains(field.Tag.Get("validate"), "required") && missing {
			return errors.New("required field missing")
		}
	}
	return nil
}

func ValidateWithContext(ctx context.Context, resource any) error {
	if Hook != nil {
		return Hook(ctx, resource)
	}
	if Reject.Load() {
		return errors.New("rejected by validator")
	}
	return nil
}

type Mode string

func ValidateRequest(value any, _ Mode) error { return ValidateResource(value) }
func ValidateRequestWithContext(ctx context.Context, value any, _ Mode) error {
	return ValidateWithContext(ctx, value)
}
func ValidateWithContextMode(ctx context.Context, value any, _ Mode) error {
	return ValidateWithContext(ctx, value)
}

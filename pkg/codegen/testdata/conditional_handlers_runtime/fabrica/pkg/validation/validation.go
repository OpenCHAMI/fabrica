// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package validation

import (
	"context"
	"errors"
	"sync/atomic"
)

var Reject atomic.Bool

// Hook exercises application validation against the generated storage layer.
var Hook func(context.Context, any) error

func ValidateResource(any) error {
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

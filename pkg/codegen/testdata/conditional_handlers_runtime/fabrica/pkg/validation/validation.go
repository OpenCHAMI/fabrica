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

func ValidateResource(any) error {
	return nil
}

func ValidateWithContext(context.Context, any) error {
	if Reject.Load() {
		return errors.New("rejected by validator")
	}
	return nil
}

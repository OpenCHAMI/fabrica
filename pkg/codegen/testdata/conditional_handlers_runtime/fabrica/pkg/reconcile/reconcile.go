// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

// Package reconcile supplies the client contract used by generated storage fixtures.
package reconcile

import "context"

type ClientInterface interface {
	Get(context.Context, string, string) (interface{}, error)
	List(context.Context, string) ([]interface{}, error)
	Update(context.Context, interface{}) error
	Create(context.Context, interface{}) error
	Delete(context.Context, string, string) error
}

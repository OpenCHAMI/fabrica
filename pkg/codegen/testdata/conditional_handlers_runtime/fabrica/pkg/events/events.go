// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"sync/atomic"
)

var Mutations atomic.Int32

func PublishResourceCreated(context.Context, string, string, string, any) error {
	Mutations.Add(1)
	return nil
}

func PublishResourceUpdated(context.Context, string, string, string, any, map[string]any) error {
	Mutations.Add(1)
	return nil
}

func PublishResourcePatched(context.Context, string, string, string, any, map[string]any) error {
	Mutations.Add(1)
	return nil
}

func PublishResourceDeleted(context.Context, string, string, string, map[string]any) error {
	Mutations.Add(1)
	return nil
}

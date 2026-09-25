// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package events

import "context"

func PublishResourceCreated(context.Context, string, string, string, any) error {
	return nil
}

func PublishResourceUpdated(context.Context, string, string, string, any, map[string]any) error {
	return nil
}

func PublishResourcePatched(context.Context, string, string, string, any, map[string]any) error {
	return nil
}

func PublishResourceDeleted(context.Context, string, string, string, map[string]any) error {
	return nil
}

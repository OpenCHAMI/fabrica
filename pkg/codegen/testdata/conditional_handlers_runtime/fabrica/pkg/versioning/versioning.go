// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package versioning

import "context"

type VersionContext struct {
	GroupVersion string
	ServeVersion string
}

func GetVersionContext(context.Context) *VersionContext {
	return &VersionContext{GroupVersion: "v1", ServeVersion: "v1"}
}

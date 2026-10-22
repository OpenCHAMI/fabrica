// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import "github.com/openchami/fabrica/pkg/uid"

// NewUID returns a revision-scoped TypeID whose payload is a UUIDv7.
func NewUID() (string, error) {
	return (uid.Codec{}).Generate("rev")
}

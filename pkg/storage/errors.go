// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
// SPDX-License-Identifier: MIT

package storage

import "errors"

// ErrDedicatedMigrationRequired prevents reads/writes from silently hiding rows
// left in generic storage after a resource switches to dedicated storage.
var ErrDedicatedMigrationRequired = errors.New("dedicated storage requires migration of existing generic rows")

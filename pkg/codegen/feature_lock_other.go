//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"os"
)

func lockFeatureFile(*os.File) error {
	return fmt.Errorf("recoverable feature generation requires filesystem locking on this platform")
}
func unlockFeatureFile(*os.File)          {}
func syncFeatureDirectory(*os.File) error { return nil }

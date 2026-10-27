//go:build windows

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"golang.org/x/sys/windows"
	"os"
)

func lockFeatureFile(f *os.File) error {
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &windows.Overlapped{})
}
func unlockFeatureFile(f *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &windows.Overlapped{})
}

// Windows does not provide portable directory fsync through os.File. Individual
// staged files are flushed before rename; power-loss durability is filesystem-specific.
func syncFeatureDirectory(*os.File) error { return nil }

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"os"
	"syscall"
)

func lockFeatureFile(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}
func unlockFeatureFile(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
func syncFeatureDirectory(f *os.File) error {
	err := f.Sync()
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
		return nil
	}
	return err
}

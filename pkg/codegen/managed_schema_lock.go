// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
)

// ErrManagedSchemaBusy identifies a concurrent generator that owns the managed schema lock.
var ErrManagedSchemaBusy = errors.New("managed schema transaction is busy")

// ManagedSchemaBusyError records the canonical schema root whose lock is held.
type ManagedSchemaBusyError struct {
	CanonicalRoot string
}

func (e *ManagedSchemaBusyError) Error() string {
	return fmt.Sprintf("managed schema root %s is locked", e.CanonicalRoot)
}

// Is reports whether target identifies the managed-schema busy error family.
func (e *ManagedSchemaBusyError) Is(target error) bool {
	return target == ErrManagedSchemaBusy
}

type managedSchemaFileLock interface {
	TryLock() (bool, error)
	Unlock() error
}

func newManagedSchemaKernelLock(path string) managedSchemaFileLock {
	return flock.New(path, flock.SetPermissions(0o600))
}

type rootMutexEntry struct {
	mutex sync.Mutex
	refs  int
}

type rootMutexRegistry struct {
	mutex sync.Mutex
	roots map[string]*rootMutexEntry
}

var managedSchemaRootMutexes = rootMutexRegistry{roots: make(map[string]*rootMutexEntry)}

func (r *rootMutexRegistry) acquire(root string) func() {
	r.mutex.Lock()
	entry := r.roots[root]
	if entry == nil {
		entry = &rootMutexEntry{}
		r.roots[root] = entry
	}
	entry.refs++
	r.mutex.Unlock()
	entry.mutex.Lock()
	return func() {
		entry.mutex.Unlock()
		r.mutex.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(r.roots, root)
		}
		r.mutex.Unlock()
	}
}

func (o *managedSchemaOutput) withTransactionLock(action func() error) error {
	release, err := o.acquireTransactionLock()
	if err != nil {
		return err
	}
	return errors.Join(action(), release())
}

func (o *managedSchemaOutput) acquireTransactionLock() (func() error, error) {
	if err := o.initializeCanonicalRoot(); err != nil {
		return nil, err
	}
	releaseLocal := managedSchemaRootMutexes.acquire(o.canonical)
	if err := o.ops.mkdirAll(filepath.Dir(o.current), 0o755); err != nil {
		releaseLocal()
		return nil, fmt.Errorf("create schema lock parent: %w", err)
	}
	if err := o.validateLockPath(); err != nil {
		releaseLocal()
		return nil, err
	}
	fileLock := o.ops.newFileLock(o.lock)
	locked, err := fileLock.TryLock()
	if err != nil {
		releaseLocal()
		return nil, fmt.Errorf("acquire schema file lock %s: %w", o.lock, err)
	}
	if !locked {
		releaseLocal()
		return nil, &ManagedSchemaBusyError{CanonicalRoot: o.canonical}
	}
	if err := o.validateAcquiredLockPath(); err != nil {
		unlockErr := fileLock.Unlock()
		releaseLocal()
		return nil, errors.Join(err, unlockErr)
	}
	return func() error {
		err := fileLock.Unlock()
		releaseLocal()
		if err != nil {
			return fmt.Errorf("release schema file lock %s: %w", o.lock, err)
		}
		return nil
	}, nil
}

func (o *managedSchemaOutput) validateLockPath() error {
	info, err := o.ops.lstat(o.lock)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect schema lock path %s: %w", o.lock, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("schema lock path %s is protected: %w", o.lock, errUnmanagedSchemaPath)
	}
	return nil
}

func (o *managedSchemaOutput) validateAcquiredLockPath() error {
	info, err := o.ops.lstat(o.lock)
	if err != nil {
		return fmt.Errorf("inspect acquired schema lock path %s: %w", o.lock, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("acquired schema lock path %s is protected: %w", o.lock, errUnmanagedSchemaPath)
	}
	return nil
}

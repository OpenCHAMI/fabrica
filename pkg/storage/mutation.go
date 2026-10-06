// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Mutation computes a replacement or deletion from authoritative stored data.
// Returning an error leaves storage unchanged. Backends may retry the callback;
// it must not perform external side effects or call the backend recursively.
type Mutation func(json.RawMessage) (replacement json.RawMessage, remove bool, err error)

// AtomicMutationBackend is an optional capability. Implementations must serialize
// Mutate with every Save and Delete, including writes through other connections.
// Deletions return the removed representation; updates return the committed one.
type AtomicMutationBackend interface {
	Mutate(context.Context, string, string, Mutation) (json.RawMessage, error)
}

var _ AtomicMutationBackend = (*FileBackend)(nil)

// ErrAtomicMutationUnsupported identifies a backend missing conditional-write support.
var ErrAtomicMutationUnsupported = errors.New("storage backend does not support atomic conditional mutations")

// MutateResource adapts an atomic backend to a typed resource callback.
func MutateResource[T any](ctx context.Context, backend StorageBackend, kind, uid string, fn func(T) (bool, error)) (T, error) {
	var result T
	atomic, ok := backend.(AtomicMutationBackend)
	if !ok {
		return result, ErrAtomicMutationUnsupported
	}
	raw, err := atomic.Mutate(ctx, kind, uid, func(raw json.RawMessage) (json.RawMessage, bool, error) {
		var current T
		if err := json.Unmarshal(raw, &current); err != nil {
			return nil, false, err
		}
		remove, err := fn(current)
		if err != nil || remove {
			return nil, remove, err
		}
		next, err := json.Marshal(current)
		return next, false, err
	})
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result)
	return result, err
}

// WithFileMutation serializes snapshot operations in one server process. All
// writers of the same directory must use this guard; fn must not re-enter it.
func WithFileMutation(ctx context.Context, directory string, fn func() error) error {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	path, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	guard, _ := fileBackendLocks.LoadOrStore(path, &sync.RWMutex{})
	mu := guard.(*sync.RWMutex)
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn()
}

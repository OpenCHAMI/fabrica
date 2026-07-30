// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"os"
	"path/filepath"
)

func (o *managedSchemaOutput) canonicalizeRoot() (string, error) {
	abs, err := o.ops.abs(o.current)
	if err != nil {
		return "", fmt.Errorf("absolute schema root: %w", err)
	}
	if info, err := o.ops.lstat(abs); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("schema root %s is not a real directory: %w", abs, errUnmanagedSchemaPath)
		}
		canonical, err := o.ops.evalSymlinks(abs)
		if err != nil {
			return "", fmt.Errorf("canonicalize schema root %s: %w", abs, err)
		}
		return canonical, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect schema root %s: %w", abs, err)
	}
	return o.canonicalizeMissingRoot(abs)
}

func (o *managedSchemaOutput) canonicalizeMissingRoot(abs string) (string, error) {
	cursor := abs
	missing := make([]string, 0, 4)
	for {
		info, err := o.ops.lstat(cursor)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("schema root ancestor %s is not a directory: %w", cursor, errUnmanagedSchemaPath)
			}
			canonical, err := o.ops.evalSymlinks(cursor)
			if err != nil {
				return "", fmt.Errorf("canonicalize schema root ancestor %s: %w", cursor, err)
			}
			for index := len(missing) - 1; index >= 0; index-- {
				canonical = filepath.Join(canonical, missing[index])
			}
			return canonical, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect schema root ancestor %s: %w", cursor, err)
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			return "", fmt.Errorf("no existing ancestor for schema root %s", abs)
		}
		missing = append(missing, filepath.Base(cursor))
		cursor = parent
	}
}

func (o *managedSchemaOutput) initializeCanonicalRoot() error {
	canonical, err := o.canonicalizeRoot()
	if err != nil {
		return err
	}
	o.canonical = canonical
	o.setArtifactPaths(canonical)
	return nil
}

func (o *managedSchemaOutput) validateCurrentRoot() error {
	canonical, err := o.canonicalizeRoot()
	if err != nil {
		return err
	}
	if canonical != o.canonical {
		return fmt.Errorf("schema root identity changed from %s to %s: %w", o.canonical, canonical, errUnmanagedSchemaPath)
	}
	return nil
}

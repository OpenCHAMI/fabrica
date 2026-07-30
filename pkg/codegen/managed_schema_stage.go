// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func (o *managedSchemaOutput) copyUnmanagedTree() error {
	if err := o.validateCurrentRoot(); err != nil {
		if _, statErr := o.ops.lstat(o.current); os.IsNotExist(statErr) {
			return nil
		}
		return err
	}
	if _, err := o.ops.lstat(o.current); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect current schema tree: %w", err)
	}
	return o.copyUnmanagedDirectory(o.current, o.stagingTree())
}

func (o *managedSchemaOutput) copyUnmanagedDirectory(source, destination string) error {
	entries, err := o.ops.readDir(source)
	if err != nil {
		return fmt.Errorf("read schema directory %s: %w", source, err)
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		info, err := o.ops.lstat(sourcePath)
		if err != nil {
			return fmt.Errorf("inspect schema path %s: %w", sourcePath, err)
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := o.ops.readlink(sourcePath)
			if err != nil {
				return fmt.Errorf("read schema symlink %s: %w", sourcePath, err)
			}
			if err := o.ops.symlink(target, destinationPath); err != nil {
				return fmt.Errorf("preserve schema symlink %s: %w", sourcePath, err)
			}
		case info.IsDir():
			if err := o.ops.mkdirAll(destinationPath, info.Mode().Perm()); err != nil {
				return fmt.Errorf("create preserved schema directory %s: %w", destinationPath, err)
			}
			if err := o.copyUnmanagedDirectory(sourcePath, destinationPath); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			content, err := o.ops.readFile(sourcePath)
			if err != nil {
				return fmt.Errorf("read schema file %s: %w", sourcePath, err)
			}
			if isFabricaManagedSchema(content) {
				continue
			}
			if err := o.ops.writeFile(destinationPath, content, info.Mode().Perm()); err != nil {
				return fmt.Errorf("preserve schema file %s: %w", sourcePath, err)
			}
		default:
			return fmt.Errorf("unsupported schema path mode for %s: %w", sourcePath, errUnmanagedSchemaPath)
		}
	}
	return nil
}

func schemaPathExists(lstat func(string) (fs.FileInfo, error), path string) (bool, error) {
	_, err := lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

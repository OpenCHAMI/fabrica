// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func snapshotAtomicTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[relative] = bytes.Clone(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot output tree: %v", err)
	}
	return result
}

func snapshotGeneratedManagedTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	for _, relativeRoot := range []string{"internal", "pkg/resources"} {
		for path, content := range snapshotAtomicTree(t, filepath.Join(root, relativeRoot)) {
			result[filepath.Join(relativeRoot, path)] = content
		}
	}
	return result
}

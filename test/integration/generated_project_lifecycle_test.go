// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	generatedProjectFixturePrefix = "fabrica-generated-project-fixture-"
	generatedProjectLegacyPrefix  = "fabrica-generated-project-cache-"
	generatedProjectOwnerFile     = ".fabrica-harness-owner"
	generatedProjectRetentionEnv  = "FABRICA_TEST_RETAIN_FAILED_FIXTURES"
	generatedProjectStaleAge      = 24 * time.Hour
)

func newGeneratedProjectRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("", generatedProjectFixturePrefix)
	if err != nil {
		t.Fatalf("create generated-project fixture: %v", err)
	}
	owner, err := currentGeneratedProjectOwner()
	if err != nil {
		_ = os.RemoveAll(root)
		t.Fatalf("resolve generated-project owner: %v", err)
	}
	if err := writeGeneratedProjectOwner(root, owner); err != nil {
		_ = os.RemoveAll(root)
		t.Fatalf("mark generated-project fixture ownership: %v", err)
	}
	retainFailed := generatedProjectRetentionEnabled()
	t.Cleanup(func() {
		if err := cleanupGeneratedProjectFixture(root, retainFailed, t.Failed(), t.Logf); err != nil {
			t.Errorf("cleanup generated-project fixture: %v", err)
		}
	})
	return root
}

func generatedProjectRetentionEnabled() bool {
	return os.Getenv(generatedProjectRetentionEnv) == "1"
}

func cleanupGeneratedProjectFixture(root string, retain, failed bool, logf func(string, ...any)) error {
	if retain && failed {
		logf("retained failed generated-project fixture: %s", root)
		return nil
	}
	return os.RemoveAll(root)
}

func currentGeneratedProjectOwner() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("current user: %w", err)
	}
	if current.Uid == "" {
		return "", errors.New("current user has no UID")
	}
	return current.Uid, nil
}

func writeGeneratedProjectOwner(root, owner string) error {
	return os.WriteFile(filepath.Join(root, generatedProjectOwnerFile), []byte(owner+"\n"), 0o600)
}

func scavengeStaleGeneratedProjectCaches(parent string, now time.Time, owner string) error {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return fmt.Errorf("read temp root: %w", err)
	}
	var cleanupErrors []error
	for _, entry := range entries {
		if !generatedProjectHarnessName(entry.Name()) || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("inspect %s: %w", entry.Name(), err))
			continue
		}
		if !info.IsDir() || info.Mode().Perm()&0o077 != 0 || now.Sub(info.ModTime()) < generatedProjectStaleAge {
			continue
		}
		root := filepath.Join(parent, entry.Name())
		marker, err := os.ReadFile(filepath.Join(root, generatedProjectOwnerFile))
		if err != nil || strings.TrimSpace(string(marker)) != owner {
			continue
		}
		if err := os.RemoveAll(root); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove stale harness cache %s: %w", root, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func generatedProjectHarnessName(name string) bool {
	return strings.HasPrefix(name, generatedProjectHarnessPrefix) ||
		strings.HasPrefix(name, generatedProjectFixturePrefix) ||
		strings.HasPrefix(name, generatedProjectLegacyPrefix)
}

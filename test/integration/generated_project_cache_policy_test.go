// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeneratedProjectEnvironment_reuses_default_module_cache(t *testing.T) {
	// Given
	environment := generatedProjectEnvironment()

	// When
	values := make(map[string]string)
	for _, item := range environment {
		key, value, found := strings.Cut(item, "=")
		if found {
			values[key] = value
		}
	}

	// Then
	if _, overridden := values["GOMODCACHE"]; overridden {
		t.Fatalf("generated-project environment overrides GOMODCACHE: %s", values["GOMODCACHE"])
	}
	wantBuildCache := filepath.Join(generatedProjectCacheRoot, "build")
	if values["GOCACHE"] != wantBuildCache {
		t.Fatalf("generated-project GOCACHE = %q, want isolated %q", values["GOCACHE"], wantBuildCache)
	}
	if _, err := os.Stat(wantBuildCache); err != nil && !os.IsNotExist(err) {
		t.Fatalf("inspect isolated build cache: %v", err)
	}
}

func TestGeneratedProjectFixtureCleanup_retains_only_failed_opt_in_fixture(t *testing.T) {
	tests := []struct {
		name   string
		retain bool
		failed bool
		kept   bool
	}{
		{name: "ordinary success removes fixture"},
		{name: "ordinary failure removes fixture", failed: true},
		{name: "opt-in success removes fixture", retain: true},
		{name: "opt-in failure retains fixture", retain: true, failed: true, kept: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			root, err := os.MkdirTemp(t.TempDir(), generatedProjectFixturePrefix)
			if err != nil {
				t.Fatalf("create fixture policy directory: %v", err)
			}
			var log strings.Builder

			// When
			err = cleanupGeneratedProjectFixture(root, test.retain, test.failed, func(format string, args ...any) {
				_, _ = fmt.Fprintf(&log, format, args...)
			})

			// Then
			if err != nil {
				t.Fatalf("cleanup generated fixture: %v", err)
			}
			_, statErr := os.Stat(root)
			if test.kept != (statErr == nil) {
				t.Fatalf("fixture retained = %t, want %t (stat error: %v)", statErr == nil, test.kept, statErr)
			}
			if test.kept && !strings.Contains(log.String(), root) {
				t.Fatalf("retention log %q does not include %s", log.String(), root)
			}
		})
	}
}

func TestGeneratedProjectRetention_requires_explicit_env_value(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset value", value: ""},
		{name: "truthy word is not accepted", value: "true"},
		{name: "explicit one enables retention", value: "1", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(generatedProjectRetentionEnv, test.value)
			if got := generatedProjectRetentionEnabled(); got != test.want {
				t.Fatalf("retention enabled = %t, want %t for %q", got, test.want, test.value)
			}
		})
	}
}

func TestGeneratedProjectCacheScavenger_removes_only_owned_stale_directories(t *testing.T) {
	// Given
	parent := t.TempDir()
	now := time.Now().UTC()
	owner := "test-owner"
	staleOwned := createHarnessCacheFixture(t, parent, generatedProjectHarnessPrefix+"stale-owned", owner, now.Add(-48*time.Hour))
	freshOwned := createHarnessCacheFixture(t, parent, generatedProjectHarnessPrefix+"fresh-owned", owner, now)
	staleForeign := createHarnessCacheFixture(t, parent, generatedProjectHarnessPrefix+"stale-foreign", "other-owner", now.Add(-48*time.Hour))
	unrelated := createHarnessCacheFixture(t, parent, "unrelated-stale", owner, now.Add(-48*time.Hour))
	target := t.TempDir()
	symlink := filepath.Join(parent, generatedProjectHarnessPrefix+"stale-link")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatalf("create stale symlink: %v", err)
	}

	// When
	err := scavengeStaleGeneratedProjectCaches(parent, now, owner)

	// Then
	if err != nil {
		t.Fatalf("scavenge generated-project caches: %v", err)
	}
	if _, err := os.Stat(staleOwned); !os.IsNotExist(err) {
		t.Fatalf("owned stale cache remains: %v", err)
	}
	for _, path := range []string{freshOwned, staleForeign, unrelated, symlink, target} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("safe cache path %s was removed: %v", path, err)
		}
	}
}

func createHarnessCacheFixture(t *testing.T, parent, name, owner string, modified time.Time) string {
	t.Helper()
	root := filepath.Join(parent, name)
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatalf("create cache fixture %s: %v", name, err)
	}
	if err := writeGeneratedProjectOwner(root, owner); err != nil {
		t.Fatalf("write cache owner %s: %v", name, err)
	}
	if err := os.Chtimes(root, modified, modified); err != nil {
		t.Fatalf("age cache fixture %s: %v", name, err)
	}
	return root
}

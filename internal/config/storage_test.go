// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorageMutationMaxAttempts(t *testing.T) {
	if got := NewDefaultConfig("test", "example.com/test").Features.Storage.MutationMaxAttempts; got != 8 {
		t.Fatalf("default attempts=%d, want 8", got)
	}
	for _, tc := range []struct {
		name, setting string
		want          int
		invalid       bool
	}{
		{"omitted", "", 0, false},
		{"zero", "mutation_max_attempts: 0", 0, false},
		{"no retries", "mutation_max_attempts: 1", 1, false},
		{"custom", "mutation_max_attempts: 3", 3, false},
		{"negative", "mutation_max_attempts: -1", -1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			yaml := "project:\n  name: test\n  module: example.com/test\nfeatures:\n  storage:\n    enabled: true\n    type: ent\n    " + tc.setting + "\n"
			if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(yaml), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Features.Storage.MutationMaxAttempts != tc.want {
				t.Fatalf("loaded attempts=%d, want %d", cfg.Features.Storage.MutationMaxAttempts, tc.want)
			}
			if err := ValidateConfig(cfg); (err != nil) != tc.invalid {
				t.Fatalf("validation=%v, invalid=%v", err, tc.invalid)
			}
			if !tc.invalid {
				if err := SaveConfig(dir, cfg); err != nil {
					t.Fatal(err)
				}
				got, err := LoadConfig(dir)
				if err != nil {
					t.Fatal(err)
				}
				if got.Features.Storage.MutationMaxAttempts != tc.want {
					t.Fatalf("roundtrip attempts=%d, want %d", got.Features.Storage.MutationMaxAttempts, tc.want)
				}
			}
		})
	}
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openchami/fabrica/pkg/codegen"
	"github.com/openchami/fabrica/pkg/validation"
)

func TestValidationPolicyNormalization(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, mode := range []string{"", "strict", "warn", "disabled", "invalid"} {
			t.Run(fmt.Sprintf("%t/%s", enabled, mode), func(t *testing.T) {
				cfg := NewDefaultConfig("test", "example.com/test")
				cfg.Features.Validation = ValidationConfig{Enabled: enabled, Mode: mode}
				expected, resolveErr := validation.ResolveMode(enabled, mode)
				err := ValidateConfig(cfg)
				features := codegen.FeatureConfig{ValidationEnabled: enabled, ValidationMode: mode}
				featureErr := features.Validate()
				if (err != nil) != (resolveErr != nil) || (featureErr != nil) != (resolveErr != nil) {
					t.Fatalf("config=%v feature=%v resolver=%v", err, featureErr, resolveErr)
				}
				if err != nil {
					return
				}
				features = features.Normalize()
				if cfg.Features.Validation.Mode != string(expected) || features.ValidationMode != string(expected) || cfg.Features.Validation.Enabled != (expected != validation.Disabled) || features.ValidationEnabled != (expected != validation.Disabled) {
					t.Fatalf("inconsistent policies: config=%+v features=%+v", cfg.Features.Validation, features)
				}
			})
		}
	}
}

func TestLoadValidationDefaults(t *testing.T) {
	for _, yaml := range []string{"{}", "features:\n  validation: {}", "features:\n  validation:\n    mode: warn"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(yaml), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatal(err)
		}
		want := "strict"
		if yaml == "features:\n  validation:\n    mode: warn" {
			want = "warn"
		}
		if !cfg.Features.Validation.Enabled || cfg.Features.Validation.Mode != want {
			t.Fatalf("%s: %+v", yaml, cfg.Features.Validation)
		}
	}
}

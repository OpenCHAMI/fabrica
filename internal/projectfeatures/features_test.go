// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package projectfeatures

import (
	"github.com/openchami/fabrica/internal/config"
	"testing"
)

func TestFromConfigLegacyAndSecurityAuth(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, security := range []bool{false, true} {
			cfg := config.NewDefaultConfig("fixture", "example.com/fixture")
			cfg.Features.Auth.Enabled = legacy
			cfg.Features.Security.AuthN.Enabled = security
			f := FromConfig(cfg, true)
			if f.AuthEnabled != (legacy || security) {
				t.Fatalf("legacy=%v security=%v: %+v", legacy, security, f)
			}
			if !f.VersioningEnabled || f.ReconcileWorkers != 5 || f.ReconcileRequeueMinutes != 5 {
				t.Fatalf("defaults: %+v", f)
			}
		}
	}
}

func TestFromConfigValidationPolicy(t *testing.T) {
	defaults := FromConfig(nil, true)
	if !defaults.ValidationEnabled || defaults.ValidationMode != "strict" {
		t.Fatalf("nil config defaults: %+v", defaults)
	}
	for _, enabled := range []bool{true, false} {
		for _, mode := range []string{"", "strict", "warn", "disabled", "invalid"} {
			cfg := config.NewDefaultConfig("fixture", "example.com/fixture")
			cfg.Features.Validation = config.ValidationConfig{Enabled: enabled, Mode: mode}
			got := FromConfig(cfg, true)
			if mode == "invalid" {
				if got.Validate() == nil {
					t.Fatal("bridge discarded invalid mode")
				}
				continue
			}
			want := mode
			if want == "" {
				want = "strict"
			}
			if !enabled || mode == "disabled" {
				want = "disabled"
			}
			if got.ValidationMode != want || got.ValidationEnabled != (want != "disabled") {
				t.Fatalf("enabled=%t mode=%q: %+v", enabled, mode, got)
			}
		}
	}
}

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

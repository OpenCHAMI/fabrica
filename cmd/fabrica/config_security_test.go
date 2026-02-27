// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"testing"
)

func TestValidateConfig_AuthZRequiresAuthN(t *testing.T) {
	cfg := NewDefaultConfig("test", "example.com/test")
	cfg.Features.Security.AuthZ.Enabled = true
	cfg.Features.Security.AuthN.Enabled = false

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateConfig_InvalidAuthZModeDefaultsToEnforceAndWarns(t *testing.T) {
	var warn bytes.Buffer

	mode := normalizeSecurityMode(SecurityMode("bogus"), &warn)
	if mode != SecurityModeEnforce {
		t.Fatalf("expected %q, got %q", SecurityModeEnforce, mode)
	}
	if warn.Len() == 0 {
		t.Fatalf("expected warning output")
	}
}

func TestValidateConfig_EmptyAuthZModeDefaultsToEnforceWithoutWarning(t *testing.T) {
	var warn bytes.Buffer

	mode := normalizeSecurityMode("", &warn)
	if mode != SecurityModeEnforce {
		t.Fatalf("expected %q, got %q", SecurityModeEnforce, mode)
	}
	if warn.Len() != 0 {
		t.Fatalf("expected no warning output")
	}
}

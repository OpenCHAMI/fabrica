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

func TestCreateFabricaConfig_WithAuthEnablesSecurityAuthN(t *testing.T) {
	dir := t.TempDir()

	err := createFabricaConfig(dir, &initOptions{
		modulePath:       "example.com/test",
		withAuth:         true,
		withStorage:      true,
		storageType:      "file",
		dbDriver:         "sqlite",
		validationMode:   "strict",
		eventBusType:     "memory",
		reconcileWorkers: 5,
	})
	if err != nil {
		t.Fatalf("createFabricaConfig returned error: %v", err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if !cfg.Features.Auth.Enabled {
		t.Fatalf("expected legacy auth flag to be enabled")
	}
	if !cfg.Features.Security.AuthN.Enabled {
		t.Fatalf("expected security.authn.enabled to be enabled")
	}
	if cfg.Features.Security.AuthZ.Enabled {
		t.Fatalf("expected security.authz.enabled to remain disabled")
	}
	if cfg.Features.Security.AuthZ.Mode != SecurityModeEnforce {
		t.Fatalf("expected default authz mode %q, got %q", SecurityModeEnforce, cfg.Features.Security.AuthZ.Mode)
	}
}

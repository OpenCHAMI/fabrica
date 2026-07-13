// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"testing"

	"github.com/openchami/fabrica/internal/config"
)

func TestValidateConfig_AuthZRequiresAuthN(t *testing.T) {
	cfg := config.NewDefaultConfig("test", "example.com/test")
	cfg.Features.Security.AuthZ.Enabled = true
	cfg.Features.Security.AuthN.Enabled = false

	err := config.ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateConfig_AllowsDisabledStorage(t *testing.T) {
	cfg := config.NewDefaultConfig("test", "example.com/test")
	cfg.Features.Storage.Enabled = false

	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatalf("expected storage-disabled config to be valid: %v", err)
	}
}

func TestValidateConfig_EventsBusMemoryOnly(t *testing.T) {
	cfg := config.NewDefaultConfig("test", "example.com/test")
	cfg.Features.Events.Enabled = true
	cfg.Features.Events.BusType = "nats"

	if err := config.ValidateConfig(cfg); err == nil {
		t.Fatalf("expected non-memory event bus to be rejected")
	}
}

func TestValidateConfig_ReconciliationRequiresEvents(t *testing.T) {
	cfg := config.NewDefaultConfig("test", "example.com/test")
	cfg.Features.Reconciliation.Enabled = true
	cfg.Features.Events.Enabled = false

	if err := config.ValidateConfig(cfg); err == nil {
		t.Fatalf("expected reconciliation without events to be rejected")
	}
}

func TestValidateConfig_InvalidAuthZModeDefaultsToEnforceAndWarns(t *testing.T) {
	var warn bytes.Buffer

	mode := config.NormalizeSecurityMode(config.SecurityMode("bogus"), &warn)
	if mode != config.SecurityModeEnforce {
		t.Fatalf("expected %q, got %q", config.SecurityModeEnforce, mode)
	}
	if warn.Len() == 0 {
		t.Fatalf("expected warning output")
	}
}

func TestValidateConfig_EmptyAuthZModeDefaultsToEnforceWithoutWarning(t *testing.T) {
	var warn bytes.Buffer

	mode := config.NormalizeSecurityMode("", &warn)
	if mode != config.SecurityModeEnforce {
		t.Fatalf("expected %q, got %q", config.SecurityModeEnforce, mode)
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

	cfg, err := config.LoadConfig(dir)
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
	if cfg.Features.Security.AuthZ.Mode != config.SecurityModeEnforce {
		t.Fatalf("expected default authz mode %q, got %q", config.SecurityModeEnforce, cfg.Features.Security.AuthZ.Mode)
	}
}

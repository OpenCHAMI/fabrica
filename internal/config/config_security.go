// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package config provides shared Fabrica project configuration models and helpers.
package config

import (
	"fmt"
	"io"
)

// SecurityMode determines AuthZ enforcement behavior.
//
// Generator-driven only; not intended to be overridden at runtime.
type SecurityMode string

const (
	// SecurityModeEnforce causes generated authorization to reject unauthorized requests.
	SecurityModeEnforce SecurityMode = "enforce"
	// SecurityModeShadow causes generated authorization to observe and report denials without enforcing them.
	SecurityModeShadow SecurityMode = "shadow"
)

// SecurityConfig controls TokenSmith-first authentication/authorization generation.
//
// NOTE: This is generator configuration, not runtime configuration.
type SecurityConfig struct {
	AuthN AuthNConfig `yaml:"authn"`
	AuthZ AuthZConfig `yaml:"authz"`
}

// AuthNConfig controls whether authentication support is generated.
type AuthNConfig struct {
	Enabled bool `yaml:"enabled"`
}

// AuthZConfig controls whether authorization is generated and how it behaves.
type AuthZConfig struct {
	Enabled bool         `yaml:"enabled"`
	Mode    SecurityMode `yaml:"mode"`
}

func (m SecurityMode) valid() bool {
	switch m {
	case SecurityModeEnforce, SecurityModeShadow:
		return true
	default:
		return false
	}
}

// NormalizeSecurityMode coerces empty or invalid modes to the default enforce mode.
func NormalizeSecurityMode(mode SecurityMode, warnOut io.Writer) SecurityMode {
	if mode == "" {
		return SecurityModeEnforce
	}
	if mode.valid() {
		return mode
	}
	if warnOut != nil {
		fmt.Fprintf(warnOut, "warning: invalid security.authz.mode %q; defaulting to %q\n", mode, SecurityModeEnforce) //nolint:errcheck
	}
	return SecurityModeEnforce
}

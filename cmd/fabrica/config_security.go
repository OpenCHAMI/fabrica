// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io"
)

// SecurityMode determines AuthZ enforcement behavior.
//
// Generator-driven only; not intended to be overridden at runtime.
type SecurityMode string

const (
	SecurityModeEnforce SecurityMode = "enforce"
	SecurityModeShadow  SecurityMode = "shadow"
)

// SecurityConfig controls TokenSmith-first authentication/authorization generation.
//
// NOTE: This is generator configuration, not runtime configuration.
type SecurityConfig struct {
	AuthN AuthNConfig `yaml:"authn"`
	AuthZ AuthZConfig `yaml:"authz"`
}

type AuthNConfig struct {
	Enabled bool `yaml:"enabled"`
}

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

func normalizeSecurityMode(mode SecurityMode, warnOut io.Writer) SecurityMode {
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

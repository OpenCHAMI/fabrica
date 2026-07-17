// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"testing"
)

func TestParseDirective(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantParts []string
		wantOpts  map[string]string
		wantErr   bool
	}{
		{
			name:      "simple directive",
			input:     "+fabrica:field:storage",
			wantParts: []string{"field", "storage"},
			wantOpts:  map[string]string{},
		},
		{
			name:      "directive with options",
			input:     "+fabrica:field:storage=hashed:bcrypt:cost=12",
			wantParts: []string{"field", "storage=hashed", "bcrypt", "cost=12"},
			wantOpts:  map[string]string{"storage": "hashed", "cost": "12"},
		},
		{
			name:      "with comment prefix",
			input:     "// +fabrica:field:index",
			wantParts: []string{"field", "index"},
			wantOpts:  map[string]string{},
		},
		{
			name:    "empty after prefix",
			input:   "+fabrica:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := ParseDirective(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDirective() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Check parts
			if len(dir.Parts()) != len(tt.wantParts) {
				t.Errorf("Parts() = %v, want %v", dir.Parts(), tt.wantParts)
			} else {
				for i, part := range tt.wantParts {
					if dir.GetPart(i) != part {
						t.Errorf("GetPart(%d) = %v, want %v", i, dir.GetPart(i), part)
					}
				}
			}

			// Check opts
			if len(dir.Opts()) != len(tt.wantOpts) {
				t.Errorf("Opts() = %v, want %v", dir.Opts(), tt.wantOpts)
			} else {
				for k, v := range tt.wantOpts {
					if got, ok := dir.GetOpt(k); !ok || got != v {
						t.Errorf("GetOpt(%q) = %v, %v, want %v, true", k, got, ok, v)
					}
				}
			}
		})
	}
}

func TestDirective_GetOptInt(t *testing.T) {
	dir, _ := ParseDirective("+fabrica:field:cost=12")

	cost, err := dir.GetOptInt("cost", 10)
	if err != nil {
		t.Errorf("GetOptInt() error = %v", err)
	}
	if cost != 12 {
		t.Errorf("GetOptInt() = %v, want 12", cost)
	}

	// Test default
	other, err := dir.GetOptInt("other", 99)
	if err != nil {
		t.Errorf("GetOptInt() error = %v", err)
	}
	if other != 99 {
		t.Errorf("GetOptInt() = %v, want 99", other)
	}
}

func TestDirective_GetOptBool(t *testing.T) {
	tests := []struct {
		input      string
		key        string
		defaultVal bool
		want       bool
		wantErr    bool
	}{
		{"+fabrica:field:active=true", "active", false, true, false},
		{"+fabrica:field:active=false", "active", true, false, false},
		{"+fabrica:field:active=yes", "active", false, true, false},
		{"+fabrica:field:active=no", "active", true, false, false},
		{"+fabrica:field:active=1", "active", false, true, false},
		{"+fabrica:field:active=0", "active", true, false, false},
		{"+fabrica:field:other=value", "active", true, true, false},   // default
		{"+fabrica:field:active=invalid", "active", true, true, true}, // error
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			dir, _ := ParseDirective(tt.input)
			got, err := dir.GetOptBool(tt.key, tt.defaultVal)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOptBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetOptBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

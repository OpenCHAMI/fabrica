// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import "testing"

func TestParseVersion_GitDescribeSuffix(t *testing.T) {
	major, minor, patch, err := parseVersion("v0.4.4-4-g1c64d98")
	if err != nil {
		t.Fatalf("expected git-describe version to parse, got error: %v", err)
	}
	if major != 0 || minor != 4 || patch != 4 {
		t.Fatalf("expected 0.4.4, got %d.%d.%d", major, minor, patch)
	}
}

func TestParseVersion_BuildMetadata(t *testing.T) {
	major, minor, patch, err := parseVersion("v1.2.3+build.7")
	if err != nil {
		t.Fatalf("expected build metadata version to parse, got error: %v", err)
	}
	if major != 1 || minor != 2 || patch != 3 {
		t.Fatalf("expected 1.2.3, got %d.%d.%d", major, minor, patch)
	}
}

func TestParseVersion_DevSentinel(t *testing.T) {
	major, minor, patch, err := parseVersion("dev")
	if err != nil {
		t.Fatalf("expected dev sentinel to parse, got error: %v", err)
	}
	if major != 0 || minor != 0 || patch != 0 {
		t.Fatalf("expected 0.0.0, got %d.%d.%d", major, minor, patch)
	}
}

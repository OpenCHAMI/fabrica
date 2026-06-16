// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import "testing"

func TestCheckVersionCompatibility_AllowsEqualGitDescribeVersion(t *testing.T) {
	ok, err := checkVersionCompatibility("v0.4.4-6-g88dc4ca-dirty", "v0.4.4-6-g88dc4ca-dirty", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !ok {
		t.Fatal("expected compatibility check to allow equal versions")
	}
}

func TestCheckVersionCompatibility_BlocksNewerGeneratedCode(t *testing.T) {
	ok, err := checkVersionCompatibility("v0.4.4", "v0.4.5", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ok {
		t.Fatal("expected compatibility check to block newer generated code")
	}
}

func TestCheckVersionCompatibility_AllowsEqualReleaseVersion(t *testing.T) {
	ok, err := checkVersionCompatibility("v0.4.4", "v0.4.4", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !ok {
		t.Fatal("expected compatibility check to allow equal release versions")
	}
}

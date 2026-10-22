// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceUIDPrefixScaffolding(t *testing.T) {
	for _, tc := range []struct {
		name      string
		versioned bool
		want      string
	}{
		{"Device", true, "device"}, {"Device", false, "dev"},
		{"VM", true, "vm"}, {"VM", false, "vm"},
		{"X", false, "x"}, {"AB_Cd", true, "ab_cd"},
	} {
		got, err := resourceUIDPrefix(tc.name, tc.versioned)
		if err != nil || got != tc.want {
			t.Fatalf("resourceUIDPrefix(%q, %t) = %q, %v", tc.name, tc.versioned, got, err)
		}
	}
	for _, tc := range []struct {
		name      string
		versioned bool
	}{
		{"", true}, {"Node1", true}, {"Nöde", true}, {"_Node", true},
		{"Node_", true}, {strings.Repeat("A", 64), true},
		{"AB_Cd", false}, // legacy three-byte prefix would end in underscore
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "resource.go")
			if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := generateResourceFile(path, tc.name, tc.versioned, &addOptions{version: "v1"}, "example.com/test", "v1", "example.com"); err == nil {
				t.Fatal("scaffolding accepted incompatible prefix")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "original" {
				t.Fatalf("invalid prefix modified resource file: %q, %v", got, err)
			}
		})
	}
}

func TestRunnerValidatesPrefixesBeforeEmittingServerFiles(t *testing.T) {
	runner := generateRunnerCode("/tmp/project", "example.com/demo", "cmd/server", "main", true, true, true, false, false, "file")
	preflight := strings.Index(runner, "if err := gen.ValidateTypeIDPrefixes();")
	if preflight < 0 {
		t.Fatal("runner has no prefix preflight")
	}
	for _, method := range []string{"GenerateHandlers", "GenerateEntSchemas", "GenerateOpenAPI", "GenerateRoutes"} {
		if at := strings.Index(runner, "gen."+method+"()"); at < preflight {
			t.Fatalf("%s runs before prefix validation", method)
		}
	}
}

func TestShortLegacyResourceScaffolding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vm.go")
	if err := generateResourceFile(path, "VM", false, &addOptions{packageName: "vm", withStatus: true}, "", "", ""); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(content), `resource.RegisterResourcePrefix("VM", "vm")`) {
		t.Fatalf("short legacy resource = %s, %v", content, err)
	}
}

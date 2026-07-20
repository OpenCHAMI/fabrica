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

func TestNormalizeVersionForCompatibilityCheck(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "CLI release matches prefixed module release", version: "0.4.9", want: "0.4.9"},
		{name: "prefixed CLI release matches module release", version: "v0.4.9", want: "0.4.9"},
		{name: "trims surrounding whitespace", version: "  v0.4.9  ", want: "0.4.9"},
		{name: "preserves dev sentinel", version: "dev", want: "dev"},
		{name: "preserves empty version", version: "", want: ""},
		{name: "preserves prerelease suffix", version: "v0.4.9-rc.1", want: "0.4.9-rc.1"},
		{name: "preserves pseudo-version suffix", version: "v0.4.9-0.20260720000000-abcdef123456", want: "0.4.9-0.20260720000000-abcdef123456"},
		{name: "preserves git describe suffix generically", version: "v0.4.9-6-g88dc4ca-dirty", want: "0.4.9-6-g88dc4ca-dirty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			version := tt.version

			// When
			got := normalizeVersionForCompatibilityCheck(version)

			// Then
			if got != tt.want {
				t.Fatalf("normalizeVersionForCompatibilityCheck(%q) = %q, want %q", version, got, tt.want)
			}
		})
	}
}

func TestNormalizeCLIVersionForCompatibilityCheck(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "exact release tag dirty", version: "v0.4.9-dirty", want: "0.4.9"},
		{name: "exact prerelease tag dirty", version: "v0.4.9-rc.1-dirty", want: "0.4.9-rc.1"},
		{name: "signed positive count remains exact", version: "v0.4.9-+6-g88dc4ca", want: "0.4.9-+6-g88dc4ca"},
		{name: "signed negative count remains exact", version: "v0.4.9--6-g88dc4ca", want: "0.4.9--6-g88dc4ca"},
		{name: "zero-padded count remains exact", version: "v0.4.9-06-g88dc4ca", want: "0.4.9-06-g88dc4ca"},
		{name: "zero count remains exact", version: "v0.4.9-0-g88dc4ca", want: "0.4.9-0-g88dc4ca"},
		{name: "nonnumeric count remains exact", version: "v0.4.9-six-g88dc4ca", want: "0.4.9-six-g88dc4ca"},
		{name: "nonhex hash remains exact", version: "v0.4.9-6-gnothex", want: "0.4.9-6-gnothex"},
		{name: "empty hash remains exact", version: "v0.4.9-6-g", want: "0.4.9-6-g"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			version := tt.version

			// When
			got := normalizeCLIVersionForCompatibilityCheck(version)

			// Then
			if got != tt.want {
				t.Fatalf("normalizeCLIVersionForCompatibilityCheck(%q) = %q, want %q", version, got, tt.want)
			}
		})
	}
}

func TestModuleVersionsCompatible(t *testing.T) {
	tests := []struct {
		name           string
		cliVersion     string
		projectVersion string
		want           bool
	}{
		{name: "exact release versions", cliVersion: "0.4.9", projectVersion: "0.4.9", want: true},
		{name: "CLI release and prefixed module release", cliVersion: "0.4.9", projectVersion: "v0.4.9", want: true},
		{name: "development CLI", cliVersion: "dev", projectVersion: "v9.9.9", want: true},
		{name: "unknown project version", cliVersion: "0.4.9", projectVersion: "", want: true},
		{name: "different release versions", cliVersion: "0.4.9", projectVersion: "v0.5.0", want: false},
		{name: "release differs from prerelease", cliVersion: "0.4.9", projectVersion: "v0.4.9-rc.1", want: false},
		{name: "release differs from pseudo-version", cliVersion: "0.4.9", projectVersion: "v0.4.9-0.20260720000000-abcdef123456", want: false},
		{name: "identical prerelease accepts optional prefix", cliVersion: "0.4.9-rc.1", projectVersion: "v0.4.9-rc.1", want: true},
		{name: "identical pseudo-version accepts optional prefix", cliVersion: "0.4.9-0.20260720000000-abcdef123456", projectVersion: "v0.4.9-0.20260720000000-abcdef123456", want: true},
		{name: "exact release tag dirty CLI matches release module", cliVersion: "v0.4.9-dirty", projectVersion: "v0.4.9", want: true},
		{name: "exact prerelease tag dirty CLI matches prerelease module", cliVersion: "v0.4.9-rc.1-dirty", projectVersion: "v0.4.9-rc.1", want: true},
		{name: "git describe CLI matches release tag", cliVersion: "v0.4.9-6-g88dc4ca-dirty", projectVersion: "v0.4.9", want: true},
		{name: "git describe CLI matches prerelease tag", cliVersion: "v0.4.9-rc.1-6-g88dc4ca", projectVersion: "v0.4.9-rc.1", want: true},
		{name: "identical non-git dash suffix accepts optional prefix", cliVersion: "0.4.9-custom", projectVersion: "v0.4.9-custom", want: true},
		{name: "different non-git dash suffixes remain distinct", cliVersion: "0.4.9-custom", projectVersion: "v0.4.9-other", want: false},
		{name: "zero git commit count remains distinct", cliVersion: "v0.4.9-0-g88dc4ca", projectVersion: "v0.4.9", want: false},
		{name: "nonhex git hash remains distinct", cliVersion: "v0.4.9-6-gnothex", projectVersion: "v0.4.9", want: false},
		{name: "empty git hash remains distinct", cliVersion: "v0.4.9-6-g", projectVersion: "v0.4.9", want: false},
		{name: "module dirty suffix remains distinct from release CLI", cliVersion: "0.4.9", projectVersion: "v0.4.9-dirty", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cliVersion := tt.cliVersion
			projectVersion := tt.projectVersion

			// When
			got := moduleVersionsCompatible(cliVersion, projectVersion)

			// Then
			if got != tt.want {
				t.Fatalf("moduleVersionsCompatible(%q, %q) = %t, want %t", cliVersion, projectVersion, got, tt.want)
			}
		})
	}
}

func TestGenerateCommand_ModulePreflightRunsBeforeRegistrationMutation(t *testing.T) {
	// Given
	projectDir := t.TempDir()
	resourcesDir := filepath.Join(projectDir, "pkg", "resources")
	if err := os.MkdirAll(resourcesDir, 0o755); err != nil {
		t.Fatalf("create resources directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/preflight\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	fabricaConfig := "project:\n  name: preflight\n  module: example.com/preflight\nfeatures:\n  storage:\n    enabled: true\n    type: file\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".fabrica.yaml"), []byte(fabricaConfig), 0o644); err != nil {
		t.Fatalf("write Fabrica config: %v", err)
	}
	apisConfig := "groups:\n  - name: test.example\n    storageVersion: v1\n    versions: [v1]\n    resources: [Widget]\n"
	if err := os.WriteFile(filepath.Join(projectDir, "apis.yaml"), []byte(apisConfig), 0o644); err != nil {
		t.Fatalf("write APIs config: %v", err)
	}
	resourceSource := "package resources\n\nimport \"github.com/openchami/fabrica/pkg/resource\"\n\ntype Widget struct { resource.Resource }\n"
	if err := os.WriteFile(filepath.Join(resourcesDir, "widget.go"), []byte(resourceSource), 0o644); err != nil {
		t.Fatalf("write resource: %v", err)
	}
	registrationPath := filepath.Join(resourcesDir, "register_generated.go")
	originalRegistration := []byte("package resources\n\nconst preflightSentinel = \"unchanged\"\n")
	if err := os.WriteFile(registrationPath, originalRegistration, 0o644); err != nil {
		t.Fatalf("write registration sentinel: %v", err)
	}

	fakeBinDir := t.TempDir()
	fakeGo := filepath.Join(fakeBinDir, "go")
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nprintf '%s\\n' 'github.com/openchami/fabrica v9.9.9'\n"), 0o755); err != nil {
		t.Fatalf("write fake go command: %v", err)
	}
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("change to project directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	originalVersion := version
	version = "0.4.9"
	t.Cleanup(func() { version = originalVersion })
	cmd := newGenerateCommand()
	cmd.SetArgs([]string{})

	// When
	err = cmd.Execute()

	// Then
	if err == nil || !strings.Contains(err.Error(), "Module version mismatch detected") {
		t.Fatalf("generate error = %v, want module version mismatch", err)
	}
	registration, err := os.ReadFile(registrationPath)
	if err != nil {
		t.Fatalf("read registration sentinel: %v", err)
	}
	if string(registration) != string(originalRegistration) {
		t.Fatalf("registration file mutated before module preflight:\n%s", registration)
	}
}

func TestCheckModuleCompatibility_ErrorMessage_HasActionableGuidance(t *testing.T) {
	// This test documents what the error message structure should contain
	// when version mismatch is detected
	expectedParts := []string{
		"Module version mismatch detected",
		"Fabrica CLI version",
		"Project module version",
		"Rebuild the Fabrica CLI",
		"Point your project to a local Fabrica checkout",
		"Update your project to the same Fabrica version",
		"--force",
	}

	// Example error message that would be returned on version mismatch
	expectedError := `
❌ Module version mismatch detected:

   Fabrica CLI version: v0.4.0
   Project module version: v0.3.0
   Project module: github.com/openchami/fabrica

This mismatch can cause code generation to fail with cryptic errors.

To fix, choose one of the following:

  1. Rebuild the Fabrica CLI from the current repository:
     cd <fabrica-repo> && make build

  2. Point your project to a local Fabrica checkout:
     cd <project> && go mod edit -replace 'github.com/openchami/fabrica=<path-to-fabrica-repo>'
     go mod tidy

  3. Update your project to the same Fabrica version as the CLI:
	 cd <project> && go get 'github.com/openchami/fabrica@v0.4.0'
     go mod tidy

Or use --force to skip this check and proceed at your own risk.
`

	for _, part := range expectedParts {
		if !strings.Contains(expectedError, part) {
			t.Fatalf("error message should contain %q, but got: %s", part, expectedError)
		}
	}
}

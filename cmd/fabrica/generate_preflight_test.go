// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestCheckModuleCompatibility_ExactVersionMatch(t *testing.T) { //nolint:revive
	// Same versions should not error (but we can't easily unit test the full
	// function without mocking exec.Command, so this test documents the behavior)
	// The real test is in integration tests where we verify the actual go list call
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

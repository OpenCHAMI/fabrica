// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"reflect"
	"testing"
)

func TestSecurityBaseline_valid_strict_directives_resolve_identically_when_warm(t *testing.T) {
	// Given
	resetGlobalCache(t)
	filename := writeAnnotationFixture(t, resolvedAnnotationSource)

	// When
	cold, err := ResolveStorageIntent(filename, "Widget", DialectPostgreSQL)
	if err != nil {
		t.Fatalf("cold ResolveStorageIntent() error = %v", err)
	}
	warm, err := ResolveStorageIntent(filename, "Widget", DialectPostgreSQL)

	// Then
	if err != nil {
		t.Fatalf("warm ResolveStorageIntent() error = %v", err)
	}
	if !reflect.DeepEqual(cold, warm) {
		t.Fatalf("warm intent differs from cold intent\ncold: %#v\nwarm: %#v", cold, warm)
	}
	secret := resolvedFieldByName(t, warm, "Secret")
	if !secret.Sensitive || secret.Transform.Kind != TransformBcrypt || secret.Transform.BcryptCost != 12 {
		t.Fatalf("strict security intent = %#v", secret)
	}
}

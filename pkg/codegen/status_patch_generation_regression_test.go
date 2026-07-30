// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestGeneratedPatchStatus_preserves_generic_file_reload_and_dedicated_status_path(t *testing.T) {
	// Given
	gen := NewGenerator(t.TempDir(), "main", "example.com/status")
	gen.Version = "test"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	base := ResourceMetadata{
		Name: "Token", PluralName: "tokens", Package: "example.com/status/apis/example.io/v1",
		PackageAlias: "v1", TypeName: "*v1.Token", SpecType: "v1.TokenSpec",
		StatusType: "v1.TokenStatus", URLPath: "/tokens", StorageName: "Token",
		Tags: map[string]string{"versioning": "enabled"},
	}
	render := func(resource ResourceMetadata) string {
		t.Helper()
		var output bytes.Buffer
		if err := gen.Templates["handlers"].Execute(&output, gen.templateData(resource, "server/handlers.go.tmpl")); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}

	// When
	generic := render(base)
	dedicatedResource := base
	dedicatedResource.Annotations = &annotations.ResourceAnnotations{StorageMode: annotations.StorageModeDedicated}
	dedicated := render(dedicatedResource)

	// Then
	reload := "if current, err := storage.LoadToken(r.Context(), uid); err == nil {\n\t\tres.Status.Version = current.Status.Version\n\t}"
	if !strings.Contains(generic, reload) {
		t.Fatalf("generic/file PatchStatus lost authoritative version reload:\n%s", generic)
	}
	if strings.Contains(dedicated, reload) || !strings.Contains(dedicated, "storage.SaveTokenStatus(r.Context(), res)") {
		t.Fatalf("dedicated PatchStatus did not retain status-only branch:\n%s", dedicated)
	}
	if strings.Contains(generic, "failed to reload persisted Token") || strings.Contains(generic, "persistedToken") {
		t.Fatalf("generic/file CRUD response behavior changed:\n%s", generic)
	}
	if got := strings.Count(dedicated, `errors.New("failed to reload persisted Token")`); got != 3 {
		t.Fatalf("dedicated stable reload guards=%d, want 3\n%s", got, dedicated)
	}
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestGenerateRevisionReferenceModels(t *testing.T) {
	outDir := t.TempDir()
	gen := NewGenerator(outDir, "main", "example.com/revisions")
	gen.Resources = []ResourceMetadata{{
		Name:               "BootConfig",
		PluralName:         "bootConfigs",
		Package:            "example.com/revisions/apis/v1",
		PackageAlias:       "v1",
		TypeName:           "*v1.BootConfig",
		SpecType:           "v1.BootConfigSpec",
		StatusType:         "v1.BootConfigStatus",
		URLPath:            "/boot-configs",
		StorageName:        "BootConfig",
		RevisioningEnabled: true,
		BareNameSelector:   "default",
	}}

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if err := gen.GenerateModels(); err != nil {
		t.Fatalf("GenerateModels() error = %v", err)
	}
	if err := gen.GenerateClient(); err != nil {
		t.Fatalf("GenerateClient() error = %v", err)
	}

	for _, file := range []string{"models_generated.go", "client_generated.go"} {
		generated, err := os.ReadFile(filepath.Join(outDir, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		content := string(generated)
		for _, marker := range []string{
			"type ReferenceSelector = revision.Selector",
			"ReferenceSelectorDefault = revision.SelectorDefault",
			"ReferenceSelectorLatest",
			"revision.SelectorLatest",
			"type ResourceReference = revision.Reference",
			"type RevisionMetadata = revision.Metadata",
			"type RevisionSeriesStatus = revision.SeriesStatus",
			"type RevisionResolveResponse = revision.ResolveResponse",
			"type RevisionConflictResponse = revision.ConflictResponse",
			"type RevisionPreconditionResponse = revision.PreconditionResponse",
			"RevisionUIDExample",
			"revision.UIDExample",
		} {
			if !strings.Contains(content, marker) {
				t.Errorf("%s missing %q", file, marker)
			}
		}
		if strings.Contains(content, "EnsureBootConfigRevision") {
			t.Errorf("%s exposes revision endpoints in foundations-only generation", file)
		}
	}
}

func TestRevisionReferenceDefaultsBareNameToDefault(t *testing.T) {
	reference := revision.Reference{Name: "production"}

	if err := reference.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got, want := reference.SelectorOrDefault(), revision.SelectorDefault; got != want {
		t.Fatalf("SelectorOrDefault() = %q, want %q", got, want)
	}
}

func TestRevisionReferenceRejectsMultipleSelectors(t *testing.T) {
	reference := revision.Reference{
		Name:         "production",
		UID:          revision.UIDExample,
		RevisionName: "release-2026-09",
	}

	err := reference.Validate()
	if !errors.Is(err, revision.ErrInvalidReference) {
		t.Fatalf("Validate() error = %v, want ErrInvalidReference", err)
	}
}

func TestRevisionReferenceRejectsUnsupportedSelector(t *testing.T) {
	reference := revision.Reference{Name: "production", Selector: "newest"}

	err := reference.Validate()
	if !errors.Is(err, revision.ErrUnsupportedSelector) {
		t.Fatalf("Validate() error = %v, want ErrUnsupportedSelector", err)
	}
}

func TestRevisionUIDExampleDoesNotChangeResourceUIDFormat(t *testing.T) {
	if !strings.HasPrefix(revision.UIDExample, "rev_") {
		t.Fatalf("revision UID example = %q, want rev_ TypeID prefix", revision.UIDExample)
	}
	if !regexp.MustCompile(`^rev_[0-9a-hjkmnp-tv-z]{26}$`).MatchString(revision.UIDExample) {
		t.Fatalf("revision UID example = %q, want TypeID-compatible value", revision.UIDExample)
	}

	uid, err := resource.GenerateUID("dev")
	if err != nil {
		t.Fatalf("GenerateUID() error = %v", err)
	}
	if !strings.HasPrefix(uid, "dev-") {
		t.Fatalf("resource UID = %q, want existing dev- prefix format", uid)
	}
}

func TestRevisionMetadataIncludesSeriesLinkage(t *testing.T) {
	metadata := revision.Metadata{
		UID:        revision.UIDExample,
		SeriesName: "production",
	}

	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var wire struct {
		SeriesName string `json:"seriesName"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got, want := wire.SeriesName, "production"; got != want {
		t.Fatalf("seriesName = %q, want %q", got, want)
	}
}

func TestGenerateRevisionModelsOnlyForEnabledResources(t *testing.T) {
	outDir := t.TempDir()
	gen := NewGenerator(outDir, "main", "example.com/revisions")
	gen.Resources = []ResourceMetadata{{
		Name:         "Node",
		PluralName:   "nodes",
		Package:      "example.com/revisions/apis/v1",
		PackageAlias: "v1",
		TypeName:     "*v1.Node",
		SpecType:     "v1.NodeSpec",
		StatusType:   "v1.NodeStatus",
		URLPath:      "/nodes",
		StorageName:  "Node",
	}}

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if err := gen.GenerateModels(); err != nil {
		t.Fatalf("GenerateModels() error = %v", err)
	}

	generated, err := os.ReadFile(filepath.Join(outDir, "models_generated.go"))
	if err != nil {
		t.Fatalf("read models_generated.go: %v", err)
	}
	if strings.Contains(string(generated), "ReferenceSelector") {
		t.Fatal("disabled resource generated revision models")
	}
}

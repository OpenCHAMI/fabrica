// SPDX-FileCopyrightText: © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"testing"
	"time"

	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestRevisionContentDigestIgnoresMapOrdering(t *testing.T) {
	first := revision.Content[map[string]any, map[string]any]{
		APIVersion: "boot.openchami.dev/v1",
		Kind:       "BootConfig",
		Metadata: resource.Metadata{
			Name:        "production",
			Labels:      map[string]string{"region": "west", "tier": "prod"},
			Annotations: map[string]string{"owner": "platform", "source": "gitops"},
		},
		Spec: map[string]any{
			"kernel": "https://example.test/kernel",
			"params": map[string]any{"console": "ttyS0", "quiet": false},
		},
	}
	second := revision.Content[map[string]any, map[string]any]{
		APIVersion: "boot.openchami.dev/v1",
		Kind:       "BootConfig",
		Metadata: resource.Metadata{
			Name:        "production",
			Labels:      map[string]string{"tier": "prod", "region": "west"},
			Annotations: map[string]string{"source": "gitops", "owner": "platform"},
		},
		Spec: map[string]any{
			"params": map[string]any{"quiet": false, "console": "ttyS0"},
			"kernel": "https://example.test/kernel",
		},
	}

	firstDigest, err := revision.ContentDigest(first)
	if err != nil {
		t.Fatalf("ContentDigest(first) error = %v", err)
	}
	secondDigest, err := revision.ContentDigest(second)
	if err != nil {
		t.Fatalf("ContentDigest(second) error = %v", err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("semantic equivalents produced different digests: %q != %q", firstDigest, secondDigest)
	}
}

func TestRevisionContentDigestChangesWithSpec(t *testing.T) {
	content := revision.Content[map[string]string, struct{}]{
		APIVersion: "boot.openchami.dev/v1",
		Kind:       "BootConfig",
		Metadata:   resource.Metadata{Name: "production"},
		Spec:       map[string]string{"kernel": "v1"},
	}

	firstDigest, err := revision.ContentDigest(content)
	if err != nil {
		t.Fatalf("ContentDigest(first) error = %v", err)
	}
	content.Spec["kernel"] = "v2"
	secondDigest, err := revision.ContentDigest(content)
	if err != nil {
		t.Fatalf("ContentDigest(second) error = %v", err)
	}
	if firstDigest == secondDigest {
		t.Fatalf("changed spec produced unchanged digest %q", firstDigest)
	}
}

func TestRevisionContentDigestExcludesServerManagedFieldsAndStatus(t *testing.T) {
	first := revision.Content[map[string]string, map[string]string]{
		APIVersion: "boot.openchami.dev/v1",
		Kind:       "BootConfig",
		Metadata: resource.Metadata{
			Name:      "production",
			UID:       "legacy-resource-uid",
			CreatedAt: time.Unix(1, 0),
			UpdatedAt: time.Unix(2, 0),
		},
		Spec:   map[string]string{"kernel": "v1"},
		Status: map[string]string{"phase": "Pending"},
	}
	second := first
	second.Metadata.UID = revision.UIDExample
	second.Metadata.CreatedAt = time.Unix(3, 0)
	second.Metadata.UpdatedAt = time.Unix(4, 0)
	second.Status = map[string]string{"phase": "Ready"}

	firstDigest, err := revision.ContentDigest(first)
	if err != nil {
		t.Fatalf("ContentDigest(first) error = %v", err)
	}
	secondDigest, err := revision.ContentDigest(second)
	if err != nil {
		t.Fatalf("ContentDigest(second) error = %v", err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("excluded fields changed digest: %q != %q", firstDigest, secondDigest)
	}
}

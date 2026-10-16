// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/openchami/fabrica/pkg/resource"
)

// Content is a resource envelope used to compute an immutable revision digest.
// Status and server-managed metadata are accepted so callers can pass an envelope,
// but ContentDigest deliberately excludes them from the canonical projection.
type Content[Spec, Status any] struct {
	APIVersion string
	Kind       string
	Metadata   resource.Metadata
	Spec       Spec
	Status     Status
}

type digestMetadata struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type digestContent[Spec any] struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   digestMetadata `json:"metadata"`
	Spec       Spec           `json:"spec"`
}

// ContentDigest returns a SHA-256 digest of API identity, user-managed metadata,
// and spec. encoding/json provides deterministic map-key ordering, so equivalent
// object maps produce identical canonical bytes regardless of insertion order.
func ContentDigest[Spec, Status any](content Content[Spec, Status]) (string, error) {
	canonical, err := json.Marshal(digestContent[Spec]{
		APIVersion: content.APIVersion,
		Kind:       content.Kind,
		Metadata: digestMetadata{
			Name:        content.Metadata.Name,
			Labels:      content.Metadata.Labels,
			Annotations: content.Metadata.Annotations,
		},
		Spec: content.Spec,
	})
	if err != nil {
		return "", fmt.Errorf("revision: marshal canonical content: %w", err)
	}

	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

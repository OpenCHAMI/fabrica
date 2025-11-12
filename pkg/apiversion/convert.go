// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package apiversion

// Hub is a marker interface implemented by hub (storage) versions
// This allows runtime checks to determine if a type is a hub version
type Hub interface {
	IsHub()
}

// Convertible defines the interface for types that can be converted to/from hub
type Convertible interface {
	// ConvertTo converts this version to the hub version
	ConvertTo(hub Hub) error

	// ConvertFrom converts from the hub version to this version
	ConvertFrom(hub Hub) error
}

// Metadata represents the common metadata fields in all versions
type Metadata struct {
	Name        string            `json:"name"`
	UID         string            `json:"uid,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   string            `json:"createdAt,omitempty"`
	UpdatedAt   string            `json:"updatedAt,omitempty"`
}

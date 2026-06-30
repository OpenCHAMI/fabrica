// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   fabrica.Metadata `json:"metadata"`
	Spec       TokenSpec        `json:"spec"`
	Status     TokenStatus      `json:"status,omitempty"`
}

// TokenSpec defines the desired state of a Token
type TokenSpec struct {
	// Token value - stored as bcrypt hash for security
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Value string `json:"value" validate:"required"`

	// Human-readable token name
	// +fabrica:field:index
	// +fabrica:field:unique
	Name string `json:"name" validate:"required"`

	// Optional description of token purpose
	Description string `json:"description,omitempty"`

	// Token expiration timestamp (Unix seconds)
	// +fabrica:field:index
	ExpiresAt int64 `json:"expiresAt,omitempty"`

	// Whether the token has been revoked
	// +fabrica:field:default=false
	Revoked bool `json:"revoked"`

	// Scopes/permissions this token grants
	Scopes []string `json:"scopes,omitempty"`
}

// TokenStatus defines the observed state of a Token
type TokenStatus struct {
	// Last time the token was used
	LastUsedAt int64 `json:"lastUsedAt,omitempty"`

	// Number of times the token has been used
	UseCount int `json:"useCount"`

	// Current state: active, expired, revoked
	State string `json:"state"`
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"time"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// ProfileBinding binds nodes or nodesets to a profile.
type ProfileBinding struct {
	APIVersion string               `json:"apiVersion"`
	Kind       string               `json:"kind"`
	Metadata   fabrica.Metadata     `json:"metadata"`
	Spec       ProfileBindingSpec   `json:"spec" validate:"required"`
	Status     ProfileBindingStatus `json:"status,omitempty"`
}

// ProfileBindingTarget defines which nodes are bound.
type ProfileBindingTarget struct {
	NodeUIDs   []string      `json:"nodeUIDs,omitempty"`
	NodeSetUID string        `json:"nodeSetUID,omitempty"`
	Xnames     []string      `json:"xnames,omitempty"`
	Selector   *NodeSelector `json:"selector,omitempty"`
}

// ProfileBindingSpec defines the desired binding.
type ProfileBindingSpec struct {
	Profile      string               `json:"profile" validate:"required"`
	Target       ProfileBindingTarget `json:"target" validate:"required"`
	BootProfile  string               `json:"bootProfile,omitempty"`
	ConfigGroups []string             `json:"configGroups,omitempty"`
}

// ProfileBindingStatus reports resolved bindings and application state.
type ProfileBindingStatus struct {
	ResolvedXnames []string            `json:"resolvedXnames,omitempty"`
	AppliedAt      time.Time           `json:"appliedAt,omitempty"`
	Conditions     []fabrica.Condition `json:"conditions,omitempty"`
}

// Validate implements custom validation logic for ProfileBinding.
func (r *ProfileBinding) Validate(ctx context.Context) error {
	return nil
}

// GetKind returns the kind of the resource.
func (r *ProfileBinding) GetKind() string {
	return "ProfileBinding"
}

// GetName returns the name of the resource.
func (r *ProfileBinding) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource.
func (r *ProfileBinding) GetUID() string {
	return r.Metadata.UID
}

// IsHub marks this as the hub/storage version.
func (r *ProfileBinding) IsHub() {}

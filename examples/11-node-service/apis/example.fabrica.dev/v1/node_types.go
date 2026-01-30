// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"time"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// Node represents a composed view of inventory + boot + metadata.
// Explicit fields improve Go autodoc and versioned conversions.
type Node struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   fabrica.Metadata `json:"metadata"`
	Spec       NodeSpec         `json:"spec" validate:"required"`
	Status     NodeStatus       `json:"status,omitempty"`
}

// NodeSpec defines desired inventory and config intent.
type NodeSpec struct {
	Xname           string            `json:"xname" validate:"required"`
	Role            string            `json:"role,omitempty"`
	Subrole         string            `json:"subrole,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	InventoryGroups []string          `json:"inventoryGroups,omitempty"`

	// Optional hints
	Profile      string   `json:"profile,omitempty"`
	BootProfile  string   `json:"bootProfile,omitempty"`
	ConfigGroups []string `json:"configGroups,omitempty"`
}

// NodeStatus reports resolved/effective state from upstream systems.
type NodeStatus struct {
	EffectiveProfile      string              `json:"effectiveProfile,omitempty"`
	EffectiveBootProfile  string              `json:"effectiveBootProfile,omitempty"`
	EffectiveConfigGroups []string            `json:"effectiveConfigGroups,omitempty"`
	ResolvedBy            string              `json:"resolvedBy,omitempty"`
	ObservedAt            time.Time           `json:"observedAt,omitempty"`
	Conditions            []fabrica.Condition `json:"conditions,omitempty"`
}

// Validate implements custom validation logic for Node.
func (r *Node) Validate(ctx context.Context) error {
	return nil
}

// GetKind returns the kind of the resource.
func (r *Node) GetKind() string {
	return "Node"
}

// GetName returns the name of the resource.
func (r *Node) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource.
func (r *Node) GetUID() string {
	return r.Metadata.UID
}

// IsHub marks this as the hub/storage version.
func (r *Node) IsHub() {}

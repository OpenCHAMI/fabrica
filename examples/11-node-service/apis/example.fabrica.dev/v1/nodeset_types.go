// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"time"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// NodeSet defines a reusable selector for nodes.
type NodeSet struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   fabrica.Metadata `json:"metadata"`
	Spec       NodeSetSpec      `json:"spec" validate:"required"`
	Status     NodeSetStatus    `json:"status,omitempty"`
}

// NodeSelector selects nodes by labels, explicit xnames, or partitions.
type NodeSelector struct {
	Labels     map[string]string `json:"labels,omitempty"`
	Partitions []string          `json:"partitions,omitempty"`
	Xnames     []string          `json:"xnames,omitempty"`
	Count      int               `json:"count,omitempty" validate:"omitempty,min=0"`
	Percent    int               `json:"percent,omitempty" validate:"omitempty,min=0,max=100"`
}

// NodeSetSpec defines desired selection and optional leasing semantics.
type NodeSetSpec struct {
	Selector             NodeSelector `json:"selector" validate:"required"`
	LeaseName            string       `json:"leaseName,omitempty"`
	LeaseDurationSeconds int          `json:"leaseDurationSeconds,omitempty"`
}

// NodeSetStatus reports resolved node identities.
type NodeSetStatus struct {
	ResolvedXnames []string            `json:"resolvedXnames,omitempty"`
	Conflicts      []string            `json:"conflicts,omitempty"`
	LeaseExpiresAt time.Time           `json:"leaseExpiresAt,omitempty"`
	ObservedAt     time.Time           `json:"observedAt,omitempty"`
	Conditions     []fabrica.Condition `json:"conditions,omitempty"`
}

// Validate implements custom validation logic for NodeSet.
func (r *NodeSet) Validate(ctx context.Context) error {
	return nil
}

// GetKind returns the kind of the resource.
func (r *NodeSet) GetKind() string {
	return "NodeSet"
}

// GetName returns the name of the resource.
func (r *NodeSet) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource.
func (r *NodeSet) GetUID() string {
	return r.Metadata.UID
}

// IsHub marks this as the hub/storage version.
func (r *NodeSet) IsHub() {}

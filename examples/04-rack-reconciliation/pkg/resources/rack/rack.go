// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package rack

import (
	"github.com/alexlovelltroy/fabrica/pkg/resource"
)

// Rack represents a physical rack in a data center
type Rack struct {
	resource.Resource `json:",inline"`
	Spec              RackSpec   `json:"spec"`
	Status            RackStatus `json:"status"`
}

// RackSpec defines the desired state of Rack
type RackSpec struct {
	// Reference to RackTemplate
	TemplateUID string `json:"templateUID" validate:"required"`

	// Physical location
	Location string `json:"location"`

	// Data center
	Datacenter string `json:"datacenter,omitempty"`

	// Row and position
	Row      string `json:"row,omitempty"`
	Position string `json:"position,omitempty"`
}

// RackStatus represents the observed state of Rack
type RackStatus struct {
	// Phase of rack provisioning
	Phase string `json:"phase"` // Pending, Provisioning, Ready, Error

	// List of chassis UIDs
	ChassisUIDs []string `json:"chassisUIDs,omitempty"`

	// Total counts
	TotalChassis int `json:"totalChassis"`
	TotalBlades  int `json:"totalBlades"`
	TotalNodes   int `json:"totalNodes"`
	TotalBMCs    int `json:"totalBMCs"`

	// Conditions
	Conditions []resource.Condition `json:"conditions,omitempty"`
}

// GetKind returns the kind of the resource
func (r *Rack) GetKind() string {
	return "Rack"
}

// GetName returns the name of the resource
func (r *Rack) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *Rack) GetUID() string {
	return r.Metadata.UID
}

func init() {
	resource.RegisterResourcePrefix("Rack", "rack")
}

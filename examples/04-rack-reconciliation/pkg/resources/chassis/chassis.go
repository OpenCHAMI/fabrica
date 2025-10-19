// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package chassis

import (
	"github.com/alexlovelltroy/fabrica/pkg/resource"
)

// Chassis represents a chassis containing blades
type Chassis struct {
	resource.Resource `json:",inline"`
	Spec              ChassisSpec   `json:"spec"`
	Status            ChassisStatus `json:"status"`
}

// ChassisSpec defines the desired state of Chassis
type ChassisSpec struct {
	// Parent rack UID
	RackUID string `json:"rackUID" validate:"required"`

	// Chassis number in rack (0-based)
	ChassisNumber int `json:"chassisNumber" validate:"min=0"`

	// Model information
	Model        string `json:"model,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`
}

// ChassisStatus represents the observed state of Chassis
type ChassisStatus struct {
	// List of blade UIDs
	BladeUIDs []string `json:"bladeUIDs,omitempty"`

	// Power state
	PowerState string `json:"powerState,omitempty"` // On, Off, Unknown

	// Health
	Health string `json:"health,omitempty"` // OK, Warning, Critical, Unknown

	// Conditions
	Conditions []resource.Condition `json:"conditions,omitempty"`
}

// GetKind returns the kind of the resource
func (c *Chassis) GetKind() string {
	return "Chassis"
}

// GetName returns the name of the resource
func (c *Chassis) GetName() string {
	return c.Metadata.Name
}

// GetUID returns the UID of the resource
func (c *Chassis) GetUID() string {
	return c.Metadata.UID
}

func init() {
	resource.RegisterResourcePrefix("Chassis", "chas")
}

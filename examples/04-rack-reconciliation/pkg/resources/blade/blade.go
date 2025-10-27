// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package blade

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// Blade represents a blade server
type Blade struct {
	resource.Resource `json:",inline"`
	Spec              BladeSpec   `json:"spec"`
	Status            BladeStatus `json:"status"`
}

// BladeSpec defines the desired state of Blade
type BladeSpec struct {
	// Parent chassis UID
	ChassisUID string `json:"chassisUID" validate:"required"`

	// Blade number in chassis (0-based)
	BladeNumber int `json:"bladeNumber" validate:"min=0"`

	// Model information
	Model        string `json:"model,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`
}

// BladeStatus represents the observed state of Blade
type BladeStatus struct {
	// List of node UIDs
	NodeUIDs []string `json:"nodeUIDs,omitempty"`

	// List of BMC UIDs
	BMCUIDs []string `json:"bmcUIDs,omitempty"`

	// Power state
	PowerState string `json:"powerState,omitempty"` // On, Off, Unknown

	// Health
	Health string `json:"health,omitempty"` // OK, Warning, Critical, Unknown

	// Conditions
	Conditions []resource.Condition `json:"conditions,omitempty"`
}

// GetKind returns the kind of the resource
func (b *Blade) GetKind() string {
	return "Blade"
}

// GetName returns the name of the resource
func (b *Blade) GetName() string {
	return b.Metadata.Name
}

// GetUID returns the UID of the resource
func (b *Blade) GetUID() string {
	return b.Metadata.UID
}

func init() {
	resource.RegisterResourcePrefix("Blade", "blade")
}

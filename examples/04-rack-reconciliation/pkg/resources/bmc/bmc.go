// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package bmc

import (
	"github.com/alexlovelltroy/fabrica/pkg/resource"
)

// BMC represents a Baseboard Management Controller
type BMC struct {
	resource.Resource `json:",inline"`
	Spec              BMCSpec   `json:"spec"`
	Status            BMCStatus `json:"status"`
}

// BMCSpec defines the desired state of BMC
type BMCSpec struct {
	// Parent blade UID
	BladeUID string `json:"bladeUID" validate:"required"`

	// IP address
	IPAddress string `json:"ipAddress,omitempty"`

	// MAC address
	MACAddress string `json:"macAddress,omitempty"`

	// Firmware version
	FirmwareVersion string `json:"firmwareVersion,omitempty"`
}

// BMCStatus represents the observed state of BMC
type BMCStatus struct {
	// Managed node UIDs
	ManagedNodeUIDs []string `json:"managedNodeUIDs,omitempty"`

	// Connectivity
	Reachable bool `json:"reachable"`

	// Health
	Health string `json:"health,omitempty"` // OK, Warning, Critical, Unknown

	// Conditions
	Conditions []resource.Condition `json:"conditions,omitempty"`
}

// GetKind returns the kind of the resource
func (b *BMC) GetKind() string {
	return "BMC"
}

// GetName returns the name of the resource
func (b *BMC) GetName() string {
	return b.Metadata.Name
}

// GetUID returns the UID of the resource
func (b *BMC) GetUID() string {
	return b.Metadata.UID
}

func init() {
	resource.RegisterResourcePrefix("BMC", "bmc")
}

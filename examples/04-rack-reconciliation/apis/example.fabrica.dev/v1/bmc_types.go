//go:build ignore

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"github.com/openchami/fabrica/pkg/fabrica"
)

// BMC represents a Baseboard Management Controller.
type BMC struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   fabrica.Metadata `json:"metadata"`
	Spec       BMCSpec          `json:"spec"`
	Status     BMCStatus        `json:"status,omitempty"`
}

// BMCSpec defines the desired state of BMC.
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

// BMCStatus represents the observed state of BMC.
type BMCStatus struct {
	// Managed node UIDs
	ManagedNodeUIDs []string `json:"managedNodeUIDs,omitempty"`

	// Connectivity
	Reachable bool `json:"reachable"`

	// Health
	Health string `json:"health,omitempty"` // OK, Warning, Critical, Unknown

	// Conditions
	Conditions []fabrica.Condition `json:"conditions,omitempty"`
}

// GetKind returns the kind of the resource.
func (b *BMC) GetKind() string {
	return "BMC"
}

// GetName returns the name of the resource.
func (b *BMC) GetName() string {
	return b.Metadata.Name
}

// GetUID returns the UID of the resource.
func (b *BMC) GetUID() string {
	return b.Metadata.UID
}

// IsHub marks this as the hub/storage version.
func (b *BMC) IsHub() {}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package fru provides resource definitions for Field Replaceable Units (FRUs).
//
// This package defines the FRU resource type for managing hardware components
// that can be replaced in a system, such as CPUs, memory modules, storage devices,
// power supplies, and network cards. FRUs track hardware inventory, location,
// status, and lifecycle information for data center equipment management.
package fru

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// FRU represents a Field Replaceable Unit
type FRU struct { //nolint:revive
	resource.Resource `json:",inline"`
	Spec              FRUSpec   `json:"spec"`
	Status            FRUStatus `json:"status"`
}

// FRUSpec defines the desired state of FRU
type FRUSpec struct { //nolint:revive
	// FRU identification
	FRUType      string `json:"fruType"` // e.g., "CPU", "Memory", "Storage"
	SerialNumber string `json:"serialNumber"`
	PartNumber   string `json:"partNumber"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`

	// Location information
	Location FRULocation `json:"location"`

	// Relationships
	ParentUID    string   `json:"parentUID,omitempty"`    // Parent FRU
	ChildrenUIDs []string `json:"childrenUIDs,omitempty"` // Child FRUs

	// Redfish path for management
	RedfishPath string `json:"redfishPath,omitempty"`

	// Custom properties
	Properties map[string]string `json:"properties,omitempty"`
}

// FRULocation defines where the FRU is located
type FRULocation struct { //nolint:revive
	BMCUID   string `json:"bmcUID,omitempty"`  // BMC managing this FRU
	NodeUID  string `json:"nodeUID,omitempty"` // Node containing this FRU
	Rack     string `json:"rack,omitempty"`
	Chassis  string `json:"chassis,omitempty"`
	Slot     string `json:"slot,omitempty"`
	Bay      string `json:"bay,omitempty"`
	Position string `json:"position,omitempty"`
	Socket   string `json:"socket,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Port     string `json:"port,omitempty"`
}

// FRUStatus defines the observed state of FRU
type FRUStatus struct { //nolint:revive
	// Health and operational status
	Health     string `json:"health"`     // "OK", "Warning", "Critical", "Unknown"
	State      string `json:"state"`      // "Present", "Absent", "Disabled", "Unknown"
	Functional string `json:"functional"` // "Enabled", "Disabled", "Unknown"

	// Timestamps
	LastSeen    string `json:"lastSeen,omitempty"`
	LastScanned string `json:"lastScanned,omitempty"`

	// Error conditions
	Errors []string `json:"errors,omitempty"`

	// Additional status information
	Temperature float64              `json:"temperature,omitempty"`
	Power       float64              `json:"power,omitempty"`
	Metrics     map[string]float64   `json:"metrics,omitempty"`
	Conditions  []resource.Condition `json:"conditions,omitempty"`
}

// GetKind returns the kind of the resource
func (f *FRU) GetKind() string {
	return "FRU"
}

// GetName returns the name of the resource
func (f *FRU) GetName() string {
	return f.Metadata.Name
}

// GetUID returns the UID of the resource
func (f *FRU) GetUID() string {
	return f.Metadata.UID
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("FRU", "fru")
}

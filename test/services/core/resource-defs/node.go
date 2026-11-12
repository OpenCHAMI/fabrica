// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package resourcedefs

// NodeSpec is a hardware node, modeled on FRUSpec from
// examples/03-fru-service. It exercises nested structs, slices, and maps.
type NodeSpec struct {
	// Identification
	NodeType     string `json:"nodeType" validate:"required"` // e.g. "Compute", "Storage", "Management"
	SerialNumber string `json:"serialNumber"`
	PartNumber   string `json:"partNumber"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`

	// Location information
	Location NodeLocation `json:"location"`

	// Relationships
	ParentUID    string   `json:"parentUID,omitempty"`
	ChildrenUIDs []string `json:"childrenUIDs,omitempty"`

	// Redfish path for management
	RedfishPath string `json:"redfishPath,omitempty"`

	// Custom properties
	Properties map[string]string `json:"properties,omitempty"`
}

// NodeLocation defines where the node is located.
type NodeLocation struct {
	BMCUID  string `json:"bmcUID,omitempty"`
	Rack    string `json:"rack,omitempty"`
	Chassis string `json:"chassis,omitempty"`
	Slot    string `json:"slot,omitempty"`
	Bay     string `json:"bay,omitempty"`
}

// NodeStatus defines the observed state of a node.
type NodeStatus struct {
	Health     string `json:"health,omitempty"`     // "OK", "Warning", "Critical", "Unknown"
	State      string `json:"state,omitempty"`      // "Present", "Absent", "Disabled", "Unknown"
	Functional string `json:"functional,omitempty"` // "Enabled", "Disabled", "Unknown"

	LastSeen string   `json:"lastSeen,omitempty"`
	Errors   []string `json:"errors,omitempty"`

	Temperature float64            `json:"temperature,omitempty"`
	Power       float64            `json:"power,omitempty"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package fru

import (
	"context"
	"time"

	"github.com/alexlovelltroy/fabrica/pkg/resource"
)

// FRU represents a Field Replaceable Unit - a hardware component that can be replaced
type FRU struct {
	resource.Resource
	Spec   FRUSpec   `json:"spec" validate:"required"`
	Status FRUStatus `json:"status,omitempty"`
}

// FRUSpec defines the desired state of a FRU
type FRUSpec struct {
	// Type of FRU (e.g., "CPU", "Memory", "Storage", "Network", "Power", "Cooling")
	FRUType string `json:"fruType" validate:"required"`

	// Hardware identification
	SerialNumber   string `json:"serialNumber,omitempty"`
	PartNumber     string `json:"partNumber,omitempty"`
	Manufacturer   string `json:"manufacturer,omitempty"`
	Model          string `json:"model,omitempty"`
	Description    string `json:"description,omitempty"`
	Version        string `json:"version,omitempty"`
	FirmwareVersion string `json:"firmwareVersion,omitempty"`

	// Hierarchical relationships
	Parent   string   `json:"parent,omitempty"`   // UID of parent FRU
	Children []string `json:"children,omitempty"` // UIDs of child FRUs

	// Physical location information
	Location FRULocation `json:"location,omitempty"`

	// Redfish integration
	RedfishPath string `json:"redfishPath,omitempty"`

	// Custom properties for extensibility
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// FRULocation describes the physical placement of a FRU
type FRULocation struct {
	// High-level location hierarchy
	BMC     string `json:"bmc,omitempty"`     // BMC managing this FRU
	Node    string `json:"node,omitempty"`    // Node containing this FRU
	Rack    string `json:"rack,omitempty"`    // Rack identifier
	Chassis string `json:"chassis,omitempty"` // Chassis identifier

	// Detailed placement within chassis/node
	Slot    string `json:"slot,omitempty"`    // Slot number or identifier
	Bay     string `json:"bay,omitempty"`     // Bay identifier
	Socket  string `json:"socket,omitempty"`  // Socket identifier (for CPUs)
	Channel string `json:"channel,omitempty"` // Channel identifier (for memory)
	Port    string `json:"port,omitempty"`    // Port identifier (for network)

	// Additional location details
	Position    string            `json:"position,omitempty"`    // Free-form position description
	Coordinates map[string]string `json:"coordinates,omitempty"` // X/Y/Z or other coordinate system
}

// FRUStatus represents the observed state of a FRU
type FRUStatus struct {
	// Health and operational status
	Health      HealthStatus      `json:"health,omitempty"`
	State       OperationalState  `json:"state,omitempty"`
	Functional  bool              `json:"functional"`
	LastUpdated time.Time         `json:"lastUpdated,omitempty"`

	// Error tracking
	Errors     []FRUError        `json:"errors,omitempty"`
	Conditions []resource.Condition `json:"conditions,omitempty"`

	// Discovery and inventory metadata
	DiscoveredAt time.Time `json:"discoveredAt,omitempty"`
	Source       string    `json:"source,omitempty"` // Source of discovery (redfish, ipmi, etc.)

	// Performance or utilization metrics (optional)
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

// HealthStatus represents the health state of a FRU
type HealthStatus string

const (
	HealthOK       HealthStatus = "OK"
	HealthWarning  HealthStatus = "Warning"
	HealthCritical HealthStatus = "Critical"
	HealthUnknown  HealthStatus = "Unknown"
)

// OperationalState represents the operational state of a FRU
type OperationalState string

const (
	StateEnabled  OperationalState = "Enabled"
	StateDisabled OperationalState = "Disabled"
	StateStandby  OperationalState = "Standby"
	StateUnknown  OperationalState = "Unknown"
)

// FRUError represents an error condition on a FRU
type FRUError struct {
	Code        string    `json:"code"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
}

// CreateFRURequest represents a request to create a new FRU
type CreateFRURequest struct {
	Name string  `json:"name" validate:"required"`
	Spec FRUSpec `json:"spec" validate:"required"`
}

// UpdateFRURequest represents a request to update an existing FRU
type UpdateFRURequest struct {
	Spec   *FRUSpec   `json:"spec,omitempty"`
	Status *FRUStatus `json:"status,omitempty"`
}

// Validate implements custom validation logic for FRU
func (r *FRU) Validate(ctx context.Context) error {
	// Custom validation logic can be added here
	// For now, rely on struct tags
	return nil
}

// GetResourceType returns the resource type for API routing
func (f *FRU) GetResourceType() string {
	return "frus"
}

// GetDisplayName returns a human-readable name for the FRU
func (f *FRU) GetDisplayName() string {
	if f.Spec.Model != "" && f.Spec.SerialNumber != "" {
		return f.Spec.Model + " (" + f.Spec.SerialNumber + ")"
	}
	if f.Spec.Model != "" {
		return f.Spec.Model
	}
	if f.Metadata.Name != "" {
		return f.Metadata.Name
	}
	return f.Metadata.UID
}

// IsHealthy returns true if the FRU is in a healthy state
func (f *FRU) IsHealthy() bool {
	return f.Status.Health == HealthOK || f.Status.Health == ""
}

// IsOperational returns true if the FRU is operationally enabled
func (f *FRU) IsOperational() bool {
	return f.Status.State == StateEnabled && f.Status.Functional
}

// AddError adds an error to the FRU status
func (f *FRU) AddError(code, message, severity string) {
	if f.Status.Errors == nil {
		f.Status.Errors = []FRUError{}
	}

	f.Status.Errors = append(f.Status.Errors, FRUError{
		Code:      code,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
		Resolved:  false,
	})

	f.Status.LastUpdated = time.Now()
}

// ResolveError marks an error as resolved
func (f *FRU) ResolveError(code string) {
	for i := range f.Status.Errors {
		if f.Status.Errors[i].Code == code && !f.Status.Errors[i].Resolved {
			f.Status.Errors[i].Resolved = true
			now := time.Now()
			f.Status.Errors[i].ResolvedAt = &now
			f.Status.LastUpdated = now
			break
		}
	}
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("FRU", "fru")
}

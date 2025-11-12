// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1beta1

import (
	"context"
	"github.com/openchami/fabrica/pkg/fabrica"
)

// Device represents a device resource
type Device struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   fabrica.Metadata `json:"metadata"`
	Spec       DeviceSpec       `json:"spec" validate:"required"`
	Status     DeviceStatus     `json:"status,omitempty"`
}

// DeviceSpec defines the desired state of Device
type DeviceSpec struct {
	Name        string            `json:"name" validate:"required"`
	IPAddress   string            `json:"ipAddress" validate:"required,ip"`
	Location    string            `json:"location,omitempty"`
	DeviceType  string            `json:"deviceType" validate:"oneof=server switch router"`
	Tags        map[string]string `json:"tags,omitempty"` // NEW in v1beta1: Added tags
	Description string            `json:"description,omitempty" validate:"max=200"`
}

// DeviceStatus defines the observed state of Device
type DeviceStatus struct {
	Phase       string      `json:"phase,omitempty"`
	Message     string      `json:"message,omitempty"`
	Ready       bool        `json:"ready"`
	Health      string      `json:"health,omitempty" validate:"omitempty,oneof=healthy degraded unhealthy"`
	LastChecked string      `json:"lastChecked,omitempty"`
	Conditions  []Condition `json:"conditions,omitempty"` // NEW in v1beta1: Added conditions
}

// Condition represents a status condition
type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// Validate implements custom validation logic for Device
func (r *Device) Validate(ctx context.Context) error {
	// Add custom validation logic here
	// Example:
	// if r.Spec.Description == "forbidden" {
	//     return errors.New("description 'forbidden' is not allowed")
	// }

	return nil
}

// GetKind returns the kind of the resource
func (r *Device) GetKind() string {
	return "Device"
}

// GetName returns the name of the resource
func (r *Device) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *Device) GetUID() string {
	return r.Metadata.UID
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1alpha1

import (
	"context"
	v1 "github.com/example/device-api-versioned/apis/infra.example.io/v1"
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
	Description string `json:"description,omitempty" validate:"max=200"`
	// Add your spec fields here
}

// DeviceStatus defines the observed state of Device
type DeviceStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
	Ready   bool   `json:"ready"`
	// Add your status fields here
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

// ConvertTo converts this v1alpha1 Device to the hub version (v1)
func (src *Device) ConvertTo(dstRaw interface{}) error {
	dst := dstRaw.(*v1.Device)

	// TODO: Implement conversion logic from v1alpha1 to v1

	// Copy common fields
	dst.APIVersion = "infra.example.io/v1"
	dst.Kind = src.Kind
	dst.Metadata = src.Metadata

	// TODO: Convert Spec fields
	// Map fields from src.Spec to dst.Spec
	// Handle any field additions, removals, or transformations

	// TODO: Convert Status fields
	// Map fields from src.Status to dst.Status
	// Handle any field additions, removals, or transformations

	return nil
}

// ConvertFrom converts from the hub version (v1) to this v1alpha1 Device
func (dst *Device) ConvertFrom(srcRaw interface{}) error {
	src := srcRaw.(*v1.Device)

	// TODO: Implement conversion logic from v1 to v1alpha1

	// Copy common fields
	dst.APIVersion = "infra.example.io/v1alpha1"
	dst.Kind = src.Kind
	dst.Metadata = src.Metadata

	// TODO: Convert Spec fields
	// Map fields from src.Spec to dst.Spec
	// Handle any field additions, removals, or transformations
	// Drop fields that don't exist in v1alpha1

	// TODO: Convert Status fields
	// Map fields from src.Status to dst.Status
	// Handle any field additions, removals, or transformations
	// Drop fields that don't exist in v1alpha1

	return nil
}

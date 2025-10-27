// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package racktemplate

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// RackTemplate represents a template for rack configuration
type RackTemplate struct {
	resource.Resource `json:",inline"`
	Spec              RackTemplateSpec   `json:"spec"`
	Status            RackTemplateStatus `json:"status"`
}

// RackTemplateSpec defines the desired state of RackTemplate
type RackTemplateSpec struct {
	// Number of chassis in the rack
	ChassisCount int `json:"chassisCount" validate:"required,min=1,max=42"`

	// Configuration for each chassis
	ChassisConfig ChassisConfig `json:"chassisConfig"`

	// Description of the template
	Description string `json:"description,omitempty"`
}

// ChassisConfig defines the configuration for chassis in the rack
type ChassisConfig struct {
	// Number of blades per chassis
	BladeCount int `json:"bladeCount" validate:"required,min=1,max=16"`

	// Configuration for each blade
	BladeConfig BladeConfig `json:"bladeConfig"`
}

// BladeConfig defines the configuration for blades in a chassis
type BladeConfig struct {
	// Number of nodes per blade (1-8)
	NodeCount int `json:"nodeCount" validate:"required,min=1,max=8"`

	// BMC mode: "shared" (1 BMC per blade) or "dedicated" (1 BMC per node)
	BMCMode string `json:"bmcMode" validate:"required,oneof=shared dedicated"`
}

// RackTemplateStatus represents the observed state of RackTemplate
type RackTemplateStatus struct {
	// Total number of racks using this template
	UsageCount int `json:"usageCount"`

	// Conditions
	Conditions []resource.Condition `json:"conditions,omitempty"`
}

// GetKind returns the kind of the resource
func (r *RackTemplate) GetKind() string {
	return "RackTemplate"
}

// GetName returns the name of the resource
func (r *RackTemplate) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *RackTemplate) GetUID() string {
	return r.Metadata.UID
}

func init() {
	resource.RegisterResourcePrefix("RackTemplate", "rktmpl")
}

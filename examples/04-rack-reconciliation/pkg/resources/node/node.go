// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package node

import (
	"github.com/alexlovelltroy/fabrica/pkg/resource"
)

// Node represents a compute node
type Node struct {
	resource.Resource `json:",inline"`
	Spec              NodeSpec   `json:"spec"`
	Status            NodeStatus `json:"status"`
}

// NodeSpec defines the desired state of Node
type NodeSpec struct {
	// Parent blade UID
	BladeUID string `json:"bladeUID" validate:"required"`

	// Managing BMC UID
	BMCUID string `json:"bmcUID,omitempty"`

	// Node number in blade (0-based)
	NodeNumber int `json:"nodeNumber" validate:"min=0,max=7"`

	// Hardware configuration
	CPUModel string `json:"cpuModel,omitempty"`
	CPUCount int    `json:"cpuCount,omitempty"`
	MemoryGB int    `json:"memoryGB,omitempty"`
}

// NodeStatus represents the observed state of Node
type NodeStatus struct {
	// Power state
	PowerState string `json:"powerState,omitempty"` // On, Off, Unknown

	// Boot state
	BootState string `json:"bootState,omitempty"` // Booting, Ready, Off, Unknown

	// Health
	Health string `json:"health,omitempty"` // OK, Warning, Critical, Unknown

	// Conditions
	Conditions []resource.Condition `json:"conditions,omitempty"`
}

// GetKind returns the kind of the resource
func (n *Node) GetKind() string {
	return "Node"
}

// GetName returns the name of the resource
func (n *Node) GetName() string {
	return n.Metadata.Name
}

// GetUID returns the UID of the resource
func (n *Node) GetUID() string {
	return n.Metadata.UID
}

func init() {
	resource.RegisterResourcePrefix("Node", "node")
}

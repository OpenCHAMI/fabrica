// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package testdata

// Resource template definitions for rack reconciliation tests

// RackTemplateResource provides the RackTemplate resource definition
const RackTemplateResource = `// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package racktemplate

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// RackTemplate represents a template for rack configuration
type RackTemplate struct {
	resource.Resource ` + "`json:\",inline\"`" + `
	Spec              RackTemplateSpec   ` + "`json:\"spec\"`" + `
	Status            RackTemplateStatus ` + "`json:\"status\"`" + `
}

// RackTemplateSpec defines the desired state of RackTemplate
type RackTemplateSpec struct {
	// Number of chassis in the rack
	ChassisCount int ` + "`json:\"chassisCount\" validate:\"required,min=1,max=42\"`" + `

	// Configuration for each chassis
	ChassisConfig ChassisConfig ` + "`json:\"chassisConfig\"`" + `

	// Description of the template
	Description string ` + "`json:\"description,omitempty\"`" + `
}

// ChassisConfig defines the configuration for chassis in the rack
type ChassisConfig struct {
	// Number of blades per chassis
	BladeCount int ` + "`json:\"bladeCount\" validate:\"required,min=1,max=16\"`" + `

	// Configuration for each blade
	BladeConfig BladeConfig ` + "`json:\"bladeConfig\"`" + `
}

// BladeConfig defines the configuration for blades in a chassis
type BladeConfig struct {
	// Number of nodes per blade (1-8)
	NodeCount int ` + "`json:\"nodeCount\" validate:\"required,min=1,max=8\"`" + `

	// BMC mode: "shared" (1 BMC per blade) or "dedicated" (1 BMC per node)
	BMCMode string ` + "`json:\"bmcMode\" validate:\"required,oneof=shared dedicated\"`" + `
}

// RackTemplateStatus represents the observed state of RackTemplate
type RackTemplateStatus struct {
	// Total number of racks using this template
	UsageCount int ` + "`json:\"usageCount\"`" + `

	// Conditions
	Conditions []resource.Condition ` + "`json:\"conditions,omitempty\"`" + `
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
`

// RackResource provides the Rack resource definition
const RackResource = `// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package rack

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// Rack represents a physical rack in a data center
type Rack struct {
	resource.Resource ` + "`json:\",inline\"`" + `
	Spec              RackSpec   ` + "`json:\"spec\"`" + `
	Status            RackStatus ` + "`json:\"status\"`" + `
}

// RackSpec defines the desired state of Rack
type RackSpec struct {
	// Reference to RackTemplate
	TemplateUID string ` + "`json:\"templateUID\" validate:\"required\"`" + `

	// Physical location
	Location string ` + "`json:\"location\"`" + `

	// Data center
	Datacenter string ` + "`json:\"datacenter,omitempty\"`" + `

	// Row and position
	Row      string ` + "`json:\"row,omitempty\"`" + `
	Position string ` + "`json:\"position,omitempty\"`" + `
}

// RackStatus represents the observed state of Rack
type RackStatus struct {
	// Phase of rack provisioning
	Phase string ` + "`json:\"phase\"` // Pending, Provisioning, Ready, Error" + `

	// List of chassis UIDs
	ChassisUIDs []string ` + "`json:\"chassisUIDs,omitempty\"`" + `

	// Total counts
	TotalChassis int ` + "`json:\"totalChassis\"`" + `
	TotalBlades  int ` + "`json:\"totalBlades\"`" + `
	TotalNodes   int ` + "`json:\"totalNodes\"`" + `
	TotalBMCs    int ` + "`json:\"totalBMCs\"`" + `

	// Conditions
	Conditions []resource.Condition ` + "`json:\"conditions,omitempty\"`" + `
}

// GetKind returns the kind of the resource
func (r *Rack) GetKind() string {
	return "Rack"
}

// GetName returns the name of the resource
func (r *Rack) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *Rack) GetUID() string {
	return r.Metadata.UID
}

func init() {
	resource.RegisterResourcePrefix("Rack", "rack")
}
`

// ChassisResource provides the Chassis resource definition
const ChassisResource = `// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package chassis

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// Chassis represents a chassis containing blades
type Chassis struct {
	resource.Resource ` + "`json:\",inline\"`" + `
	Spec              ChassisSpec   ` + "`json:\"spec\"`" + `
	Status            ChassisStatus ` + "`json:\"status\"`" + `
}

// ChassisSpec defines the desired state of Chassis
type ChassisSpec struct {
	// Parent rack UID
	RackUID string ` + "`json:\"rackUID\" validate:\"required\"`" + `

	// Chassis number in rack (0-based)
	ChassisNumber int ` + "`json:\"chassisNumber\" validate:\"min=0\"`" + `

	// Model information
	Model        string ` + "`json:\"model,omitempty\"`" + `
	SerialNumber string ` + "`json:\"serialNumber,omitempty\"`" + `
}

// ChassisStatus represents the observed state of Chassis
type ChassisStatus struct {
	// List of blade UIDs
	BladeUIDs []string ` + "`json:\"bladeUIDs,omitempty\"`" + `

	// Power state
	PowerState string ` + "`json:\"powerState,omitempty\"` // On, Off, Unknown" + `

	// Health
	Health string ` + "`json:\"health,omitempty\"` // OK, Warning, Critical, Unknown" + `

	// Conditions
	Conditions []resource.Condition ` + "`json:\"conditions,omitempty\"`" + `
}

// GetKind returns the kind of the resource
func (c *Chassis) GetKind() string {
	return "Chassis"
}

// GetName returns the name of the resource
func (c *Chassis) GetName() string {
	return c.Metadata.Name
}

// GetUID returns the UID of the resource
func (c *Chassis) GetUID() string {
	return c.Metadata.UID
}

func init() {
	resource.RegisterResourcePrefix("Chassis", "chas")
}
`

// BladeResource provides the Blade resource definition
const BladeResource = `// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package blade

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// Blade represents a blade server
type Blade struct {
	resource.Resource ` + "`json:\",inline\"`" + `
	Spec              BladeSpec   ` + "`json:\"spec\"`" + `
	Status            BladeStatus ` + "`json:\"status\"`" + `
}

// BladeSpec defines the desired state of Blade
type BladeSpec struct {
	// Parent chassis UID
	ChassisUID string ` + "`json:\"chassisUID\" validate:\"required\"`" + `

	// Blade number in chassis (0-based)
	BladeNumber int ` + "`json:\"bladeNumber\" validate:\"min=0\"`" + `

	// Model information
	Model        string ` + "`json:\"model,omitempty\"`" + `
	SerialNumber string ` + "`json:\"serialNumber,omitempty\"`" + `
}

// BladeStatus represents the observed state of Blade
type BladeStatus struct {
	// List of node UIDs
	NodeUIDs []string ` + "`json:\"nodeUIDs,omitempty\"`" + `

	// List of BMC UIDs
	BMCUIDs []string ` + "`json:\"bmcUIDs,omitempty\"`" + `

	// Power state
	PowerState string ` + "`json:\"powerState,omitempty\"` // On, Off, Unknown" + `

	// Health
	Health string ` + "`json:\"health,omitempty\"` // OK, Warning, Critical, Unknown" + `

	// Conditions
	Conditions []resource.Condition ` + "`json:\"conditions,omitempty\"`" + `
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
`

// BMCResource provides the BMC resource definition
const BMCResource = `// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package bmc

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// BMC represents a Baseboard Management Controller
type BMC struct {
	resource.Resource ` + "`json:\",inline\"`" + `
	Spec              BMCSpec   ` + "`json:\"spec\"`" + `
	Status            BMCStatus ` + "`json:\"status\"`" + `
}

// BMCSpec defines the desired state of BMC
type BMCSpec struct {
	// Parent blade UID
	BladeUID string ` + "`json:\"bladeUID\" validate:\"required\"`" + `

	// IP address
	IPAddress string ` + "`json:\"ipAddress,omitempty\"`" + `

	// MAC address
	MACAddress string ` + "`json:\"macAddress,omitempty\"`" + `

	// Firmware version
	FirmwareVersion string ` + "`json:\"firmwareVersion,omitempty\"`" + `
}

// BMCStatus represents the observed state of BMC
type BMCStatus struct {
	// Managed node UIDs
	ManagedNodeUIDs []string ` + "`json:\"managedNodeUIDs,omitempty\"`" + `

	// Connectivity
	Reachable bool ` + "`json:\"reachable\"`" + `

	// Health
	Health string ` + "`json:\"health,omitempty\"` // OK, Warning, Critical, Unknown" + `

	// Conditions
	Conditions []resource.Condition ` + "`json:\"conditions,omitempty\"`" + `
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
`

// NodeResource provides the Node resource definition
const NodeResource = `// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package node

import (
	"github.com/openchami/fabrica/pkg/resource"
)

// Node represents a compute node
type Node struct {
	resource.Resource ` + "`json:\",inline\"`" + `
	Spec              NodeSpec   ` + "`json:\"spec\"`" + `
	Status            NodeStatus ` + "`json:\"status\"`" + `
}

// NodeSpec defines the desired state of Node
type NodeSpec struct {
	// Parent blade UID
	BladeUID string ` + "`json:\"bladeUID\" validate:\"required\"`" + `

	// Managing BMC UID
	BMCUID string ` + "`json:\"bmcUID,omitempty\"`" + `

	// Node number in blade (0-based)
	NodeNumber int ` + "`json:\"nodeNumber\" validate:\"min=0,max=7\"`" + `

	// Hardware configuration
	CPUModel  string ` + "`json:\"cpuModel,omitempty\"`" + `
	CPUCount  int    ` + "`json:\"cpuCount,omitempty\"`" + `
	MemoryGB  int    ` + "`json:\"memoryGB,omitempty\"`" + `
}

// NodeStatus represents the observed state of Node
type NodeStatus struct {
	// Power state
	PowerState string ` + "`json:\"powerState,omitempty\"` // On, Off, Unknown" + `

	// Boot state
	BootState string ` + "`json:\"bootState,omitempty\"` // Booting, Ready, Off, Unknown" + `

	// Health
	Health string ` + "`json:\"health,omitempty\"` // OK, Warning, Critical, Unknown" + `

	// Conditions
	Conditions []resource.Condition ` + "`json:\"conditions,omitempty\"`" + `
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
`

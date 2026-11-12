// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package resourcedefs

// DeviceSpec is the network device resource from
// examples/01-basic-crud. It uses the default (file) storage backend and plain
// validation tags; no storage annotations are involved.
type DeviceSpec struct {
	// Description is a human-readable note about the device.
	Description string `json:"description,omitempty" validate:"max=200"`

	// Hostname is the device's fully qualified host name.
	Hostname string `json:"hostname,omitempty"`

	// IPAddr is the device's management IP address.
	IPAddr string `json:"ipaddr,omitempty" validate:"omitempty,ip"`

	// Location is the physical location of the device.
	Location string `json:"location,omitempty"`

	// Rack is the rack identifier where the device is mounted.
	Rack string `json:"rack,omitempty"`
}

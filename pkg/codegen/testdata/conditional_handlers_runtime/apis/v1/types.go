// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package v1

import "time"

type Metadata struct {
	UID         string            `json:"uid"`
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type NodeSpec struct {
	Value string `json:"value"`
}

type NodeStatus struct {
	Version string `json:"version,omitempty"`
	Phase   string `json:"phase,omitempty"`
}

type Node struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   Metadata   `json:"metadata"`
	Spec       NodeSpec   `json:"spec"`
	Status     NodeStatus `json:"status"`
}

func (n *Node) GetUID() string  { return n.Metadata.UID }
func (n *Node) GetName() string { return n.Metadata.Name }

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package testfixtures holds resource types used to verify code generation.
//
// These deliberately live in a normal (non-test) package rather than in a
// _test.go file. The generated storage adapter imports the package each
// resource is declared in, and that import has to resolve from a *separate*
// module — the throwaway project the tests generate into. A type declared in a
// _test.go file exists only inside fabrica's own test binary, so the generated
// adapter would reference an identifier that is not there.
//
// Nothing here is part of fabrica's supported API. It exists so the adapter,
// query and storage templates can be compiled against a real Ent client.
package testfixtures

import "github.com/openchami/fabrica/pkg/resource"

// WidgetSpec is the desired state of a Widget.
type WidgetSpec struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}

// WidgetStatus is the observed state of a Widget.
type WidgetStatus struct {
	Phase string `json:"phase,omitempty"`
}

// Widget models a resource the way a real service does: embedding
// resource.Resource, which supplies APIVersion, Kind and Metadata.
//
// The generated adapter expects that embedded type in non-versioned mode. A
// resource declaring those fields directly, without the embed, produces an
// adapter that references fields the resource does not have — which compiles
// in fabrica and fails only in the generated project.
type Widget struct {
	resource.Resource
	Spec   WidgetSpec   `json:"spec"`
	Status WidgetStatus `json:"status,omitempty"`
}

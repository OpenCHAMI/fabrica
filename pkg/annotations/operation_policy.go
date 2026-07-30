// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import "fmt"

// OperationPolicy is the resolved generated HTTP surface for one resource.
type OperationPolicy struct {
	List          bool
	Get           bool
	Create        bool
	Update        bool
	Patch         bool
	Delete        bool
	StatusUpdate  bool
	StatusPatch   bool
	VersionList   bool
	VersionGet    bool
	VersionDelete bool
	Exposure      Exposure
}

// ResolveOperationPolicy expands annotation aliases and validates versioning constraints.
func ResolveOperationPolicy(resourceAnnotations *ResourceAnnotations, versioning bool) (OperationPolicy, error) {
	if resourceAnnotations == nil {
		resourceAnnotations = NewResourceAnnotations()
	}
	if err := validateOperationAnnotations(resourceAnnotations); err != nil {
		return OperationPolicy{}, err
	}

	policy := OperationPolicy{Exposure: resourceAnnotations.Exposure}
	if policy.Exposure == "" {
		policy.Exposure = ExposureDefault
	}
	if policy.Exposure == ExposurePrivate && !resourceAnnotations.VerbsExplicit {
		return policy, nil
	}

	verbs := resourceAnnotations.Verbs
	if len(verbs) == 0 {
		verbs = []OperationVerb{OperationAll}
	}
	if len(verbs) == 1 && verbs[0] == OperationNone {
		return policy, nil
	}
	if len(verbs) == 1 && verbs[0] == OperationAll {
		policy.enableAll(versioning)
		return policy, nil
	}

	for _, verb := range verbs {
		if isVersionOperation(verb) && !versioning {
			return OperationPolicy{}, fmt.Errorf("operation %q requires resource versioning", verb)
		}
		policy.enable(verb)
	}
	return policy, nil
}

// HasHTTPOperations reports whether any generated HTTP operation is enabled.
func (p OperationPolicy) HasHTTPOperations() bool {
	return p.List || p.Get || p.Create || p.Update || p.Patch || p.Delete ||
		p.StatusUpdate || p.StatusPatch || p.VersionList || p.VersionGet || p.VersionDelete
}

// IsPublicArtifact reports whether the resource belongs in public OpenAPI and clients.
func (p OperationPolicy) IsPublicArtifact() bool {
	return p.Exposure == ExposureDefault || p.Exposure == ExposurePublic || p.Exposure == ExposureProtected
}

func (p *OperationPolicy) enableAll(versioning bool) {
	p.List = true
	p.Get = true
	p.Create = true
	p.Update = true
	p.Patch = true
	p.Delete = true
	p.StatusUpdate = true
	p.StatusPatch = true
	p.VersionList = versioning
	p.VersionGet = versioning
	p.VersionDelete = versioning
}

func (p *OperationPolicy) enable(verb OperationVerb) {
	switch verb {
	case OperationList:
		p.List = true
	case OperationGet:
		p.Get = true
	case OperationCreate:
		p.Create = true
	case OperationUpdate:
		p.Update = true
	case OperationPatch:
		p.Patch = true
	case OperationDelete:
		p.Delete = true
	case OperationStatusUpdate:
		p.StatusUpdate = true
	case OperationStatusPatch:
		p.StatusPatch = true
	case OperationVersionList:
		p.VersionList = true
	case OperationVersionGet:
		p.VersionGet = true
	case OperationVersionDelete:
		p.VersionDelete = true
	case OperationAll, OperationNone:
	}
}

func isVersionOperation(verb OperationVerb) bool {
	return verb == OperationVersionList || verb == OperationVersionGet || verb == OperationVersionDelete
}

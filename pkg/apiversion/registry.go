// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package apiversion provides hub/spoke API versioning infrastructure for Fabrica.
//
// This package implements Kubebuilder-style versioning where:
//   - Each API group has one hub (storage) version
//   - Multiple spoke (external) versions can exist
//   - Automatic conversion between hub and spokes handles version negotiation
//
// Example:
//
//	registry := apiversion.NewRegistry()
//	registry.RegisterGroup(apiversion.Group{
//	    Name:           "infra.example.io",
//	    StorageVersion: "v1",
//	    Spokes:         []string{"v1alpha1", "v1beta1", "v1"},
//	})
//
//	group, ok := registry.Resolve("infra.example.io", "v1beta1")
//	if ok {
//	    fmt.Printf("Storage version: %s\n", group.StorageVersion)
//	}
package apiversion

import (
	"fmt"
	"sync"
)

// Group represents an API group with its versions
type Group struct {
	Name           string   // e.g., "infra.example.io"
	StorageVersion string   // e.g., "v1" - the hub version
	Spokes         []string // e.g., ["v1alpha1", "v1beta1", "v1"]
}

// Registry manages API groups and their versions
type Registry struct {
	mu     sync.RWMutex
	groups map[string]Group
}

// NewRegistry creates a new API version registry
func NewRegistry() *Registry {
	return &Registry{
		groups: make(map[string]Group),
	}
}

// RegisterGroup registers an API group with its versions
func (r *Registry) RegisterGroup(g Group) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groups[g.Name] = g
}

// Resolve looks up a group by name and optionally validates a version
func (r *Registry) Resolve(groupName, version string) (Group, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.groups[groupName]
	if !ok {
		return Group{}, false
	}

	// If version is specified, validate it exists in spokes
	if version != "" {
		found := false
		for _, spoke := range g.Spokes {
			if spoke == version {
				found = true
				break
			}
		}
		if !found {
			return Group{}, false
		}
	}

	return g, true
}

// GetGroups returns all registered groups
func (r *Registry) GetGroups() []Group {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groups := make([]Group, 0, len(r.groups))
	for _, g := range r.groups {
		groups = append(groups, g)
	}
	return groups
}

// IsVersionSupported checks if a version is supported for a given group
func (r *Registry) IsVersionSupported(groupName, version string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.groups[groupName]
	if !ok {
		return false
	}

	for _, spoke := range g.Spokes {
		if spoke == version {
			return true
		}
	}
	return false
}

// GetStorageVersion returns the storage version (hub) for a group
func (r *Registry) GetStorageVersion(groupName string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.groups[groupName]
	if !ok {
		return "", fmt.Errorf("group %s not found", groupName)
	}

	return g.StorageVersion, nil
}

// GetPreferredVersion returns the preferred version for a group
// This is typically the highest stable version or the storage version
func (r *Registry) GetPreferredVersion(groupName string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.groups[groupName]
	if !ok {
		return "", fmt.Errorf("group %s not found", groupName)
	}

	// Return the storage version as preferred
	return g.StorageVersion, nil
}

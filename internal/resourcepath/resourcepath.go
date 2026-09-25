// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

// Package resourcepath provides utilities for generating and validating resource paths in the service.
package resourcepath

import (
	"fmt"
	"regexp"
	"strings"
)

var pattern = regexp.MustCompile(`^/[a-z0-9]+(?:-[a-z0-9]+)*(?:/[a-z0-9]+(?:-[a-z0-9]+)*)*$`)

var reserved = map[string]struct{}{
	"/docs":           {},
	"/health":         {},
	"/openapi.json":   {},
	"/service/status": {},
}

// Default generates the default resource path for a given resource name.
// It converts the resource name to lowercase and appends an "s" at the end.
func Default(resource string) string {
	return "/" + strings.ToLower(resource) + "s"
}

// Validate checks if the given resource path is valid.
// It ensures the path matches the required pattern and is not reserved for service endpoints.
func Validate(path string) error {
	if !pattern.MatchString(path) {
		return fmt.Errorf("invalid path %q", path)
	}
	if _, ok := reserved[path]; ok {
		return fmt.Errorf("path %q is reserved for a service endpoint", path)
	}
	return nil
}

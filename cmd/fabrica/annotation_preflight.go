// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/openchami/fabrica/pkg/codegen"
)

func validateDiscoveredResourceAnnotations(resources []discoveredResource, storageType, dbDriver string) error {
	gen := codegen.NewGenerator("", "main", "")
	gen.SetStorageType(storageType)
	gen.SetDBDriver(dbDriver)
	for _, resource := range resources {
		gen.Resources = append(gen.Resources, codegen.ResourceMetadata{Name: resource.Name, SourcePath: resource.SourcePath})
	}
	return gen.PrepareResourceAnnotations()
}

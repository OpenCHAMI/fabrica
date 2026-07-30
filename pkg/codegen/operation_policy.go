// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"sort"

	"github.com/openchami/fabrica/pkg/annotations"
)

type templateImport struct {
	Path  string
	Alias string
}

func importsForResources(resources []ResourceMetadata) []templateImport {
	byPath := make(map[string]string, len(resources))
	for _, resource := range resources {
		if resource.Package != "" {
			byPath[resource.Package] = resource.PackageAlias
		}
	}
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	imports := make([]templateImport, 0, len(paths))
	for _, path := range paths {
		imports = append(imports, templateImport{Path: path, Alias: byPath[path]})
	}
	return imports
}

func resolvedResourceOperations(resource ResourceMetadata) annotations.OperationPolicy {
	operations := resource.Operations
	if operations.HasHTTPOperations() || operations.Exposure != "" {
		return operations
	}
	versioning := resource.Tags["versioning"] == "enabled"
	operations, _ = annotations.ResolveOperationPolicy(nil, versioning)
	return operations
}

func (g *Generator) operationTemplateData() map[string]interface{} {
	publicResources := make([]ResourceMetadata, 0)
	protectedResources := make([]ResourceMetadata, 0)
	internalResources := make([]ResourceMetadata, 0)
	httpResources := make([]ResourceMetadata, 0)
	openAPIResources := make([]ResourceMetadata, 0)
	clientResources := make([]ResourceMetadata, 0)
	clientModelResources := make([]ResourceMetadata, 0)
	cliResources := make([]ResourceMetadata, 0)

	for _, resource := range g.Resources {
		resource.Operations = resolvedResourceOperations(resource)
		if !resource.Operations.HasHTTPOperations() || resource.Operations.Exposure == annotations.ExposurePrivate {
			continue
		}
		httpResources = append(httpResources, resource)
		switch resource.Operations.Exposure {
		case annotations.ExposurePublic:
			publicResources = append(publicResources, resource)
		case annotations.ExposureInternal:
			internalResources = append(internalResources, resource)
		case annotations.ExposureDefault, annotations.ExposureProtected, "":
			protectedResources = append(protectedResources, resource)
		}
		if resource.Operations.IsPublicArtifact() {
			openAPIResources = append(openAPIResources, resource)
			clientResources = append(clientResources, resource)
			if resource.Operations.Create || resource.Operations.Update {
				clientModelResources = append(clientModelResources, resource)
			}
			if resourceHasCLIOperations(resource) {
				cliResources = append(cliResources, resource)
			}
		}
	}

	return map[string]interface{}{
		"PublicResources":      publicResources,
		"ProtectedResources":   protectedResources,
		"InternalResources":    internalResources,
		"HTTPResources":        httpResources,
		"OpenAPIResources":     openAPIResources,
		"ClientResources":      clientResources,
		"ClientModelResources": clientModelResources,
		"CLIResources":         cliResources,
		"HTTPImports":          importsForResources(httpResources),
		"OpenAPIImports":       importsForResources(openAPIResources),
		"ClientImports":        importsForResources(clientResources),
		"ClientModelImports":   importsForResources(clientModelResources),
	}
}

func resourceHasCLIOperations(resource ResourceMetadata) bool {
	operations := resource.Operations
	return operations.List || operations.Get || operations.Create || operations.Update || operations.Patch ||
		operations.Delete || operations.VersionList || operations.VersionGet || operations.VersionDelete
}

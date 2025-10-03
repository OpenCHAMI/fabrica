// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package codegen provides code generation for REST API resources.
//
// This package generates consistent CRUD operations, storage, and client code
// for all resource types. The goal is to eliminate boilerplate while maintaining
// type safety and consistency across the API.
//
// Architecture:
//   - Templates define the code patterns
//   - ResourceMetadata describes each resource type
//   - Generator applies templates to metadata
//   - Output is formatted Go code
//
// Usage:
//
//	generator := NewGenerator(outputDir, packageName, modulePath)
//	generator.RegisterResource(&myresource.MyResource{})
//	generator.GenerateAll()
//
// Generated artifacts:
//   - REST API handlers (CRUD operations)
//   - Storage operations (file-based persistence)
//   - HTTP client library
//   - Request/response models
//   - Route registration
//   - Authorization integration
//
// Customization:
//   - Edit templates to change generated code patterns
//   - Implement resource-specific policies
//   - Override storage methods for custom behavior
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SchemaVersion represents a specific version of a resource schema
type SchemaVersion struct {
	Version    string   // e.g., "v1", "v2beta1"
	IsDefault  bool     // Whether this is the default/storage version
	Stability  string   // "stable", "beta", "alpha"
	Deprecated bool     // Whether this version is deprecated
	SpecType   string   // Full type name for the spec (e.g., "user.UserSpec")
	StatusType string   // Full type name for the status (e.g., "user.UserStatus")
	TypeName   string   // Full type name (e.g., "*user.User")
	Package    string   // Import path for this version
	Transforms []string // List of transformations applied in this version
}

// ResourceMetadata holds metadata about a resource type for code generation
type ResourceMetadata struct {
	Name         string            // e.g., "User"
	PluralName   string            // e.g., "users"
	Package      string            // e.g., "github.com/example/app/pkg/resources/user"
	PackageAlias string            // e.g., "user"
	TypeName     string            // e.g., "*user.User"
	SpecType     string            // e.g., "user.UserSpec"
	StatusType   string            // e.g., "user.UserStatus"
	URLPath      string            // e.g., "/users"
	StorageName  string            // e.g., "User" for storage function names
	Tags         map[string]string // Additional metadata
	RequiresAuth bool              // Whether this resource requires authentication

	// Multi-version support
	Versions        []SchemaVersion // Multiple schema versions
	DefaultVersion  string          // Default schema version
	APIGroupVersion string          // API group version (e.g., "v2")
}

// Generator handles code generation for resources
type Generator struct {
	OutputDir   string
	PackageName string
	ModulePath  string
	Resources   []ResourceMetadata
	Templates   map[string]*template.Template
}

// NewGenerator creates a new code generator
func NewGenerator(outputDir, packageName, modulePath string) *Generator {
	return &Generator{
		OutputDir:   outputDir,
		PackageName: packageName,
		ModulePath:  modulePath,
		Resources:   make([]ResourceMetadata, 0),
		Templates:   make(map[string]*template.Template),
	}
}

// RegisterResource adds a resource type for code generation
func (g *Generator) RegisterResource(resourceType interface{}) error {
	t := reflect.TypeOf(resourceType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Extract resource metadata
	name := t.Name()
	pluralName := strings.ToLower(name) + "s"

	// Determine spec type name
	specTypeName := name + "Spec"

	// Determine storage function name
	storageName := name

	// Extract package path and create correct import paths
	pkgPath := t.PkgPath()
	var packageImport, typePrefix string

	// Get the last part of the package path
	parts := strings.Split(pkgPath, "/")
	if len(parts) > 0 {
		typePrefix = parts[len(parts)-1]
		packageImport = pkgPath
	} else {
		typePrefix = "resources"
		packageImport = pkgPath
	}

	// Initialize default version metadata
	defaultVersion := SchemaVersion{
		Version:    "v1",
		IsDefault:  true,
		Stability:  "stable",
		Deprecated: false,
		SpecType:   fmt.Sprintf("%s.%s", typePrefix, specTypeName),
		StatusType: fmt.Sprintf("%s.%sStatus", typePrefix, name),
		TypeName:   fmt.Sprintf("*%s.%s", typePrefix, name),
		Package:    packageImport,
		Transforms: []string{},
	}

	metadata := ResourceMetadata{
		Name:            name,
		PluralName:      pluralName,
		Package:         packageImport,
		PackageAlias:    typePrefix,
		TypeName:        fmt.Sprintf("*%s.%s", typePrefix, name),
		SpecType:        fmt.Sprintf("%s.%s", typePrefix, specTypeName),
		StatusType:      fmt.Sprintf("%s.%sStatus", typePrefix, name),
		URLPath:         fmt.Sprintf("/%s", pluralName),
		StorageName:     storageName,
		Tags:            make(map[string]string),
		Versions:        []SchemaVersion{defaultVersion},
		DefaultVersion:  "v1",
		APIGroupVersion: "v1", // Default API group version
	}

	g.Resources = append(g.Resources, metadata)
	return nil
}

// AddResourceVersion adds a new schema version to an existing resource
func (g *Generator) AddResourceVersion(resourceName string, version SchemaVersion) error {
	for i, resource := range g.Resources {
		if resource.Name == resourceName {
			// Check if version already exists
			for _, existingVersion := range resource.Versions {
				if existingVersion.Version == version.Version {
					return fmt.Errorf("version %s already exists for resource %s", version.Version, resourceName)
				}
			}

			// Add the new version
			g.Resources[i].Versions = append(g.Resources[i].Versions, version)

			// Update default if this version is marked as default
			if version.IsDefault {
				g.Resources[i].DefaultVersion = version.Version
			}

			return nil
		}
	}
	return fmt.Errorf("resource %s not found", resourceName)
}

// SetAPIGroupVersion sets the API group version for all resources
func (g *Generator) SetAPIGroupVersion(apiGroupVersion string) {
	for i := range g.Resources {
		g.Resources[i].APIGroupVersion = apiGroupVersion
	}
}

// GetResourceByName returns the metadata for a specific resource
func (g *Generator) GetResourceByName(name string) (*ResourceMetadata, bool) {
	for i, resource := range g.Resources {
		if resource.Name == name {
			return &g.Resources[i], true
		}
	}
	return nil, false
}

// EnableAuthForResource enables authentication for a specific resource type
func (g *Generator) EnableAuthForResource(resourceName string) error {
	for i, resource := range g.Resources {
		if resource.Name == resourceName {
			g.Resources[i].RequiresAuth = true
			return nil
		}
	}
	return fmt.Errorf("resource %s not found", resourceName)
}

// GenerateAll generates all code artifacts
func (g *Generator) GenerateAll() error {
	if err := g.LoadTemplates(); err != nil {
		return err
	}

	// Generate based on package type
	switch g.PackageName {
	case "main":
		// Server code - handlers, routes, models, storage, and openapi
		if err := g.GenerateModels(); err != nil {
			return err
		}
		if err := g.GenerateHandlers(); err != nil {
			return err
		}
		if err := g.GenerateRoutes(); err != nil {
			return err
		}
		if err := g.GenerateStorage(); err != nil {
			return err
		}
		if err := g.GenerateOpenAPI(); err != nil {
			return err
		}
	case "client":
		// Client code - client and models only
		if err := g.GenerateClient(); err != nil {
			return err
		}
		if err := g.GenerateClientModels(); err != nil {
			return err
		}
	case "reconcile":
		// Reconciliation code - reconcilers, registration, and event handlers
		if err := g.GenerateReconcilers(); err != nil {
			return err
		}
		if err := g.GenerateReconcilerRegistration(); err != nil {
			return err
		}
		if err := g.GenerateEventHandlers(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported package type: %s", g.PackageName)
	}

	return nil
}

// GenerateStorage generates storage operations for server
func (g *Generator) GenerateStorage() error {
	var buf bytes.Buffer
	data := struct {
		PackageName string
		ModulePath  string
		Resources   []ResourceMetadata
	}{
		PackageName: g.PackageName,
		ModulePath:  g.ModulePath,
		Resources:   g.Resources,
	}

	if err := g.Templates["storage"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute storage template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated storage code: %w", err)
	}

	// Write storage to internal/storage directory instead of output directory
	storageDir := filepath.Join("internal", "storage")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	filename := filepath.Join(storageDir, "storage_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	return nil
}

// GenerateClientModels generates models specifically for client package
func (g *Generator) GenerateClientModels() error {
	var buf bytes.Buffer
	data := struct {
		PackageName string
		ModulePath  string
		Resources   []ResourceMetadata
	}{
		PackageName: g.PackageName,
		ModulePath:  g.ModulePath,
		Resources:   g.Resources,
	}

	if err := g.Templates["clientModels"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute client models template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated client models code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "models_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write client models file: %w", err)
	}

	return nil
}

// GenerateReconcilers generates reconciler code for all resources
func (g *Generator) GenerateReconcilers() error {
	for _, resource := range g.Resources {
		var buf bytes.Buffer
		data := struct {
			ResourceMetadata
			ModulePath string
		}{
			ResourceMetadata: resource,
			ModulePath:       g.ModulePath,
		}

		if err := g.Templates["reconciler"].Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute reconciler template for %s: %w", resource.Name, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to format generated reconciler code for %s: %w", resource.Name, err)
		}

		filename := filepath.Join(g.OutputDir, fmt.Sprintf("%s_reconciler_generated.go", strings.ToLower(resource.Name)))
		if err := os.WriteFile(filename, formatted, 0644); err != nil {
			return fmt.Errorf("failed to write reconciler file for %s: %w", resource.Name, err)
		}
	}

	return nil
}

// GenerateReconcilerRegistration generates the reconciler registration code
func (g *Generator) GenerateReconcilerRegistration() error {
	var buf bytes.Buffer
	data := struct {
		Resources  []ResourceMetadata
		ModulePath string
	}{
		Resources:  g.Resources,
		ModulePath: g.ModulePath,
	}

	if err := g.Templates["reconcilerRegistration"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute reconciler registration template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated reconciler registration code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "registration_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write reconciler registration file: %w", err)
	}

	return nil
}

// GenerateEventHandlers generates cross-resource event handler code
func (g *Generator) GenerateEventHandlers() error {
	var buf bytes.Buffer
	data := struct {
		Resources  []ResourceMetadata
		ModulePath string
	}{
		Resources:  g.Resources,
		ModulePath: g.ModulePath,
	}

	if err := g.Templates["eventHandlers"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute event handlers template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated event handlers code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "event_handlers_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write event handlers file: %w", err)
	}

	return nil
}

// LoadTemplates loads code generation templates from files or embedded templates
func (g *Generator) LoadTemplates() error {
	// Try to load from templates directory first (for development)
	templateDir := filepath.Join("templates")

	templateFiles := map[string]string{
		"handlers":               "handlers.go.tmpl",
		"clientModels":           "client-models.go.tmpl",
		"routes":                 "routes.go.tmpl",
		"storage":                "storage.go.tmpl",
		"models":                 "models.go.tmpl",
		"client":                 "client.go.tmpl",
		"policies":               "policies.go.tmpl",
		"clientCmd":              "client-cmd.go.tmpl",
		"openapi":                "openapi.go.tmpl",
		"reconciler":             "reconciler.go.tmpl",
		"reconcilerRegistration": "reconciler-registration.go.tmpl",
		"eventHandlers":          "event-handlers.go.tmpl",
	}

	g.Templates = make(map[string]*template.Template)
	for name, filename := range templateFiles {
		templatePath := filepath.Join(templateDir, filename)

		// Read template content from file
		content, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %w", templatePath, err)
		}

		// Parse template with functions
		tmpl, err := template.New(name).Funcs(templateFuncs).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
		}
		g.Templates[name] = tmpl
	}

	return nil
}

// GenerateHandlers generates REST API handlers for all resources
func (g *Generator) GenerateHandlers() error {
	for _, resource := range g.Resources {
		var buf bytes.Buffer
		data := struct {
			ResourceMetadata
			ModulePath string
		}{
			ResourceMetadata: resource,
			ModulePath:       g.ModulePath,
		}

		if err := g.Templates["handlers"].Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute handlers template for %s: %w", resource.Name, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to format generated code for %s: %w", resource.Name, err)
		}

		filename := filepath.Join(g.OutputDir, fmt.Sprintf("%s_handlers_generated.go", strings.ToLower(resource.Name)))
		if err := os.WriteFile(filename, formatted, 0644); err != nil {
			return fmt.Errorf("failed to write handlers file for %s: %w", resource.Name, err)
		}
	}

	return nil
}

// GenerateClient generates API client library
func (g *Generator) GenerateClient() error {
	var buf bytes.Buffer
	data := struct {
		PackageName string
		ModulePath  string
		Resources   []ResourceMetadata
	}{
		PackageName: g.PackageName,
		ModulePath:  g.ModulePath,
		Resources:   g.Resources,
	}

	if err := g.Templates["client"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute client template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated client code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "client_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write client file: %w", err)
	}

	return nil
}

// GenerateModels generates request/response models
func (g *Generator) GenerateModels() error {
	var buf bytes.Buffer
	data := struct {
		PackageName string
		ModulePath  string
		Resources   []ResourceMetadata
	}{
		PackageName: g.PackageName,
		ModulePath:  g.ModulePath,
		Resources:   g.Resources,
	}

	if err := g.Templates["models"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute models template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated models code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "models_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write models file: %w", err)
	}

	return nil
}

// GenerateRoutes generates route registration code
func (g *Generator) GenerateRoutes() error {
	var buf bytes.Buffer
	data := struct {
		PackageName string
		ModulePath  string
		Resources   []ResourceMetadata
	}{
		PackageName: g.PackageName,
		ModulePath:  g.ModulePath,
		Resources:   g.Resources,
	}

	if err := g.Templates["routes"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute routes template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated routes code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "routes_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write routes file: %w", err)
	}

	return nil
}

// GeneratePolicies generates authorization policy interfaces and scaffolding
func (g *Generator) GeneratePolicies() error {
	var buf bytes.Buffer
	data := struct {
		PackageName string
		ModulePath  string
		Resources   []ResourceMetadata
	}{
		PackageName: g.PackageName,
		ModulePath:  g.ModulePath,
		Resources:   g.Resources,
	}

	if err := g.Templates["policies"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute policies template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated policies code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "policies_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write policies file: %w", err)
	}

	return nil
}

// GenerateClientCmd generates a Cobra-based CLI client
func (g *Generator) GenerateClientCmd() error {
	var buf bytes.Buffer
	data := struct {
		PackageName string
		ModulePath  string
		Resources   []ResourceMetadata
	}{
		PackageName: g.PackageName,
		ModulePath:  g.ModulePath,
		Resources:   g.Resources,
	}

	if err := g.Templates["clientCmd"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute client-cmd template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated client-cmd code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "main_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write client-cmd file: %w", err)
	}

	return nil
}

// GenerateOpenAPI generates OpenAPI specification code
func (g *Generator) GenerateOpenAPI() error {
	var buf bytes.Buffer
	data := struct {
		PackageName string
		ModulePath  string
		Resources   []ResourceMetadata
	}{
		PackageName: g.PackageName,
		ModulePath:  g.ModulePath,
		Resources:   g.Resources,
	}

	if err := g.Templates["openapi"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute openapi template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated openapi code: %w", err)
	}

	filename := filepath.Join(g.OutputDir, "openapi_generated.go")
	if err := os.WriteFile(filename, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write openapi file: %w", err)
	}

	return nil
}

// Template functions
var templateFuncs = template.FuncMap{
	"toLower":    strings.ToLower,
	"toUpper":    strings.ToUpper,
	"title":      cases.Title(language.English).String,
	"trimPrefix": strings.TrimPrefix,
	"camelCase": func(s string) string {
		if len(s) == 0 {
			return s
		}
		return strings.ToLower(s[:1]) + s[1:]
	},
}

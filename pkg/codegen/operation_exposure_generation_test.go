// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"go/format"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestOperationTemplateData_filters_exposure_without_removing_registry_resources(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/service")
	gen.Resources = []ResourceMetadata{
		operationTestResource("Default", annotations.OperationPolicy{List: true, Exposure: annotations.ExposureDefault}),
		operationTestResource("Public", annotations.OperationPolicy{Get: true, Exposure: annotations.ExposurePublic}),
		operationTestResource("Protected", annotations.OperationPolicy{Create: true, Exposure: annotations.ExposureProtected}),
		operationTestResource("Internal", annotations.OperationPolicy{List: true, Exposure: annotations.ExposureInternal}),
		operationTestResource("Private", annotations.OperationPolicy{Exposure: annotations.ExposurePrivate}),
	}

	data := gen.globalTemplateData("server/routes.go.tmpl")

	if got := len(data["Resources"].([]ResourceMetadata)); got != 5 {
		t.Fatalf("registry Resources length = %d, want 5", got)
	}
	if got := resourceNames(data["PublicResources"].([]ResourceMetadata)); got != "Public" {
		t.Errorf("PublicResources = %q", got)
	}
	if got := resourceNames(data["ProtectedResources"].([]ResourceMetadata)); got != "Default,Protected" {
		t.Errorf("ProtectedResources = %q", got)
	}
	if got := resourceNames(data["InternalResources"].([]ResourceMetadata)); got != "Internal" {
		t.Errorf("InternalResources = %q", got)
	}
	if got := resourceNames(data["ClientResources"].([]ResourceMetadata)); got != "Default,Public,Protected" {
		t.Errorf("ClientResources = %q", got)
	}
}

func TestOperationTemplates_emit_only_resolved_surface(t *testing.T) {
	readOnly := operationTestResource("Reader", annotations.OperationPolicy{
		List: true, Get: true, Exposure: annotations.ExposureProtected,
	})
	internal := operationTestResource("Internal", annotations.OperationPolicy{
		List: true, Exposure: annotations.ExposureInternal,
	})
	none := operationTestResource("Hidden", annotations.OperationPolicy{Exposure: annotations.ExposureDefault})
	private := operationTestResource("Private", annotations.OperationPolicy{Exposure: annotations.ExposurePrivate})
	gen := NewGenerator(t.TempDir(), "main", "example.com/service")
	gen.Resources = []ResourceMetadata{readOnly, internal, none, private}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}

	handlers := renderOperationTemplate(t, gen, "handlers", gen.templateData(readOnly, "server/handlers.go.tmpl"))
	assertContainsAll(t, handlers, "func GetReaders(", "func GetReader(")
	assertContainsNone(t, handlers, "func CreateReader(", "func UpdateReader(", "func PatchReader(", "func DeleteReader(", "func UpdateReaderStatus(")

	hiddenHandlers := renderOperationTemplate(t, gen, "handlers", gen.templateData(none, "server/handlers.go.tmpl"))
	assertContainsNone(t, hiddenHandlers, "func GetHidden", "func CreateHidden", "internal/storage")

	routes := renderOperationTemplate(t, gen, "routes", gen.globalTemplateData("server/routes.go.tmpl"))
	assertContainsAll(t, routes, "func RegisterGeneratedProtectedRoutes(", "registerReaderRoutes(r)", "func RegisterGeneratedInternalRoutes(", "registerInternalRoutes(r)")
	assertContainsNone(t, routes, "registerHiddenRoutes", "registerPrivateRoutes")

	openAPI := renderOperationTemplate(t, gen, "openapi", gen.globalTemplateData("server/openapi.go.tmpl"))
	assertContainsAll(t, openAPI, `OperationID = "listReaders"`, `OperationID = "getReader"`)
	assertContainsNone(t, openAPI, "registerInternalPaths", "registerPrivatePaths", "createReader")

	client := renderOperationTemplate(t, gen, "client", gen.globalTemplateData("client/client.go.tmpl"))
	assertContainsAll(t, client, "func (c *Client) GetReaders(", "func (c *Client) GetReader(")
	assertContainsNone(t, client, "CreateReader", "GetInternals", "Private")
}

func TestOperationTemplates_render_supported_operation_combinations(t *testing.T) {
	policies := []struct {
		name   string
		policy annotations.OperationPolicy
	}{
		{name: "none", policy: annotations.OperationPolicy{Exposure: annotations.ExposureDefault}},
		{name: "read only", policy: annotations.OperationPolicy{List: true, Get: true, Exposure: annotations.ExposureDefault}},
		{name: "status only", policy: annotations.OperationPolicy{StatusUpdate: true, StatusPatch: true, Exposure: annotations.ExposureDefault}},
		{name: "version only", policy: annotations.OperationPolicy{VersionList: true, VersionGet: true, VersionDelete: true, Exposure: annotations.ExposureDefault}},
	}

	for _, tt := range policies {
		t.Run(tt.name, func(t *testing.T) {
			resource := operationTestResource("Widget", tt.policy)
			gen := NewGenerator(t.TempDir(), "main", "example.com/service")
			gen.Resources = []ResourceMetadata{resource}
			if err := gen.LoadTemplates(); err != nil {
				t.Fatalf("LoadTemplates() error = %v", err)
			}

			renderOperationTemplate(t, gen, "handlers", gen.templateData(resource, "server/handlers.go.tmpl"))
			for name, templateName := range map[string]string{
				"routes":       "server/routes.go.tmpl",
				"models":       "server/models.go.tmpl",
				"openapi":      "server/openapi.go.tmpl",
				"client":       "client/client.go.tmpl",
				"clientModels": "client/models.go.tmpl",
				"clientCmd":    "client/cmd.go.tmpl",
			} {
				data := gen.globalTemplateData(templateName)
				if name == "clientCmd" {
					data["PackageName"] = "main"
				}
				renderOperationTemplate(t, gen, name, data)
			}
		})
	}
}

func operationTestResource(name string, policy annotations.OperationPolicy) ResourceMetadata {
	return ResourceMetadata{
		Name: name, PluralName: strings.ToLower(name) + "s",
		Package: "example.com/service/apis/v1", PackageAlias: "v1",
		TypeName: "*v1." + name, SpecType: "v1." + name + "Spec", StatusType: "v1." + name + "Status",
		URLPath: "/" + strings.ToLower(name) + "s", StorageName: name,
		Tags: map[string]string{"versioning": "enabled"}, Operations: policy,
	}
}

func resourceNames(resources []ResourceMetadata) string {
	names := make([]string, 0, len(resources))
	for _, resource := range resources {
		names = append(names, resource.Name)
	}
	return strings.Join(names, ",")
}

func renderOperationTemplate(t *testing.T, gen *Generator, name string, data map[string]interface{}) string {
	t.Helper()
	var output bytes.Buffer
	if err := gen.Templates[name].Execute(&output, data); err != nil {
		t.Fatalf("execute %s template: %v", name, err)
	}
	formatted, err := format.Source(output.Bytes())
	if err != nil {
		t.Fatalf("format %s template: %v\n%s", name, err, output.String())
	}
	return string(formatted)
}

func assertContainsAll(t *testing.T, output string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(output, value) {
			t.Errorf("generated output missing %q", value)
		}
	}
}

func assertContainsNone(t *testing.T, output string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(output, value) {
			t.Errorf("generated output unexpectedly contains %q", value)
		}
	}
}

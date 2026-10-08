// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRevisionFileBackendAPI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gen := revisionTestGenerator(dir)
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	var storage bytes.Buffer
	if err := gen.Templates["storage"].Execute(&storage, gen.globalTemplateData("storage/file.go.tmpl")); err != nil {
		t.Fatalf("render storage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "storage_generated.go"), storage.Bytes(), 0o644); err != nil {
		t.Fatalf("write rendered storage: %v", err)
	}
	generators := []struct {
		name string
		run  func() error
	}{
		{name: "handlers", run: gen.GenerateHandlers},
		{name: "routes", run: gen.GenerateRoutes},
		{name: "openapi", run: gen.GenerateOpenAPI},
		{name: "client", run: gen.GenerateClient},
	}
	for _, generator := range generators {
		if err := generator.run(); err != nil {
			t.Fatalf("Generate %s error = %v", generator.name, err)
		}
	}

	assertGeneratedMarkers(t, filepath.Join(dir, "storage_generated.go"), []string{
		"revision.NewFileStore[*v1.BootConfig]", "CreateBootConfigSeries", "EnsureBootConfigRevisionByName",
		"ResolveBootConfigRevision", "PromoteBootConfigDefault", "ListBootConfigRevisions", "RetireBootConfigRevision",
	})
	assertGeneratedMarkers(t, filepath.Join(dir, "bootconfig_handlers_generated.go"), []string{
		"CreateBootConfigSeries", "EnsureBootConfigRevision", "ResolveBootConfigRevision", "PromoteBootConfigDefault", "RetireBootConfigRevision",
		"http.StatusConflict", "http.StatusPreconditionFailed", "PublishRevisionEvent",
		"RevisionConflictResponse{", "RevisionPreconditionResponse{", "revision.ValueETag(records)",
	})
	assertGeneratedMarkers(t, filepath.Join(dir, "routes_generated.go"), []string{
		`resource.Route("/{name}"`, `series.Put("/revisions/{revisionName}"`, `series.Delete("/revisions/by-uid/{revisionUID}"`, `series.Get("/resolve"`, `series.Put("/default"`,
	})
	assertGeneratedMarkers(t, filepath.Join(dir, "openapi_generated.go"), []string{
		`/boot-configs/{name}/revisions/{revisionName}`, `/boot-configs/{name}/resolve`, `/boot-configs/{name}/default`,
		`Responses.Set("409"`, `Responses.Set("410"`, `Responses.Set("412"`, "RevisionSeriesStatus", "RevisionResolveResponse",
		`WithName("uid").WithIn("query")`, `WithName("revisionName").WithIn("query")`, `WithName("selector").WithIn("query")`,
		`ensureRevisionOp.Responses.Set("410"`, `getRevisionOp.Responses.Set("410"`, `retireRevisionOp.Responses.Set("410"`,
		`resolveRevisionOp.Responses.Set("410"`, `promoteDefaultOp.Responses.Set("410"`,
		"promoteDefaultOp.RequestBody", "PromoteBootConfigDefaultRequest",
	})
	assertGeneratedMarkers(t, filepath.Join(dir, "client_generated.go"), []string{
		"EnsureBootConfigRevision", "ListBootConfigRevisions", "GetBootConfigRevision", "RetireBootConfigRevision", "ResolveBootConfigRevision", "PromoteBootConfigDefault",
		`/boot-configs/%s/revisions/%s`, `/boot-configs/%s/resolve`, `/boot-configs/%s/default`,
		"GetBootConfigSeries", `headers.Get("ETag")`,
	})
}

func TestGenerateRevisionSurfacesOnlyForEnabledResources(t *testing.T) {
	t.Parallel()

	gen := revisionTestGenerator(t.TempDir())
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}

	for _, templateName := range []string{"routes", "openapi", "client", "clientCmd", "authzClassifier", "authzPolicy"} {
		var rendered bytes.Buffer
		data := gen.globalTemplateData("test")
		if templateName == "clientCmd" {
			data["PackageName"] = "main"
		}
		if err := gen.Templates[templateName].Execute(&rendered, data); err != nil {
			t.Fatalf("render %s: %v", templateName, err)
		}
		content := rendered.String()
		enabledMarker := "/boot-configs"
		if templateName == "clientCmd" {
			enabledMarker = "BootConfig"
		}
		if !strings.Contains(content, enabledMarker) || !strings.Contains(content, "revision") {
			t.Errorf("%s missing BootConfig revision surface", templateName)
		}
		if strings.Contains(content, "EnsureNodeRevision") || strings.Contains(content, "/nodes/{name}/revisions") || strings.Contains(content, "nodeRevision") {
			t.Errorf("%s exposes revision surface for disabled Node", templateName)
		}
	}
}

func TestGenerateRevisionCLIAndAuthorizationClasses(t *testing.T) {
	t.Parallel()

	gen := revisionTestGenerator(t.TempDir())
	gen.Config.WithAuth = true
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}

	var cli bytes.Buffer
	if err := gen.Templates["clientCmd"].Execute(&cli, gen.globalTemplateData("client/cmd.go.tmpl")); err != nil {
		t.Fatalf("render client CLI: %v", err)
	}
	for _, marker := range []string{"revisions", "ensure", "get", "retire", "series", "resolve", "promote-default", "if-match", "GetBootConfigSeries", "bootconfigRevisionSeriesCmd", `"etag": etag`} {
		if !strings.Contains(cli.String(), marker) {
			t.Errorf("generated CLI missing %q", marker)
		}
	}
	for _, marker := range []string{
		`bootconfigRevisionResolveCmd.Flags().String("selector", "", "revision selector: default or latest")`,
		`UID: uid, RevisionName: revisionName, Selector: client.ReferenceSelector(selector)`,
	} {
		if !strings.Contains(cli.String(), marker) {
			t.Errorf("generated CLI missing selector behavior %q", marker)
		}
	}
	if strings.Contains(cli.String(), `Flags().String("selector", "default"`) {
		t.Error("generated CLI implicitly combines the default selector with --uid or --revision-name")
	}

	var classifier bytes.Buffer
	if err := gen.Templates["authzClassifier"].Execute(&classifier, gen.globalTemplateData("server/authz_classifier.go.tmpl")); err != nil {
		t.Fatalf("render classifier: %v", err)
	}
	for _, action := range []string{"create-series", "create-revision", "read-revision", "retire-revision", "resolve-revision", "promote-default", "update-status"} {
		if !strings.Contains(classifier.String(), action) {
			t.Errorf("generated classifier missing action %q", action)
		}
	}
	for _, unsupportedAction := range []string{"retire-series", "delete-series"} {
		if strings.Contains(classifier.String(), unsupportedAction) {
			t.Errorf("generated classifier exposes unsupported action %q", unsupportedAction)
		}
	}

	var policy bytes.Buffer
	if err := gen.Templates["authzPolicy"].Execute(&policy, gen.globalTemplateData("authz/policy.csv.tmpl")); err != nil {
		t.Fatalf("render policy: %v", err)
	}
	for _, tuple := range []string{
		"p, role:viewer, /boot-configs/{name}, GET",
		"p, role:editor, /boot-configs/{name}, GET",
		"p, role:admin, /boot-configs/{name}, GET",
	} {
		if !strings.Contains(policy.String(), tuple) {
			t.Errorf("generated policy missing %q", tuple)
		}
	}
	for _, unsupportedAction := range []string{"retire-series", "delete-series"} {
		if strings.Contains(policy.String(), unsupportedAction) {
			t.Errorf("generated policy exposes unsupported action %q", unsupportedAction)
		}
	}
}

func TestGenerateRevisionImportExportFailsClosed(t *testing.T) {
	t.Parallel()

	gen := revisionTestGenerator(t.TempDir())
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}

	for templateName, test := range map[string]struct {
		forbidden string
		required  string
	}{
		"import": {forbidden: "storage.GetBootConfigByUID", required: "storage.GetNodeByUID"},
		"export": {forbidden: "storage.QuerybootConfigs", required: "storage.Querynodes"},
	} {
		var rendered bytes.Buffer
		if err := gen.Templates[templateName].Execute(&rendered, gen.globalTemplateData("server/"+templateName+".go.tmpl")); err != nil {
			t.Fatalf("render %s: %v", templateName, err)
		}
		if strings.Contains(rendered.String(), test.forbidden) {
			t.Errorf("generated %s uses mutable storage operation %q for revision-enabled resource", templateName, test.forbidden)
		}
		if !strings.Contains(rendered.String(), test.required) {
			t.Errorf("generated %s missing mutable-resource operation %q", templateName, test.required)
		}
	}
}

func TestGenerateRevisionEntStorageParity(t *testing.T) {
	t.Parallel()

	gen := revisionTestGenerator(t.TempDir())
	gen.SetStorageType("ent")
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}

	var storage bytes.Buffer
	if err := gen.Templates["storageEntRevisions"].Execute(&storage, gen.globalTemplateData("storage/ent_revisions.go.tmpl")); err != nil {
		t.Fatalf("render Ent revision storage: %v", err)
	}
	for _, marker := range []string{
		"CreateBootConfigSeries", "EnsureBootConfigRevisionByName", "ResolveBootConfigRevision",
		"PromoteBootConfigDefault", "RetireBootConfigRevision", "revisionseries.RevisionCountEQ",
		"revisionrecord.RevisionNameEQ", "revision.NewUID()",
	} {
		if !strings.Contains(storage.String(), marker) {
			t.Errorf("generated Ent revision storage missing %q", marker)
		}
	}

	for templateName, markers := range map[string][]string{
		"entSchemaRevisionSeries": {"default_revision_uid", "latest_revision_uid", "revision_count"},
		"entSchemaRevisionRecord": {"revision_name", "number", "uid", "retired_at"},
	} {
		var schema bytes.Buffer
		if err := gen.Templates[templateName].Execute(&schema, gen.globalTemplateData("test")); err != nil {
			t.Fatalf("render %s: %v", templateName, err)
		}
		for _, marker := range markers {
			if !strings.Contains(schema.String(), marker) {
				t.Errorf("%s missing %q", templateName, marker)
			}
		}
	}
}

func revisionTestGenerator(dir string) *Generator {
	gen := NewGenerator(dir, "main", "example.com/revisions")
	gen.Config.VersioningEnabled = true
	gen.Resources = []ResourceMetadata{
		{Name: "BootConfig", PluralName: "bootConfigs", Package: "example.com/revisions/apis/v1", PackageAlias: "v1", TypeName: "*v1.BootConfig", SpecType: "v1.BootConfigSpec", StatusType: "v1.BootConfigStatus", URLPath: "/boot-configs", StorageName: "BootConfig", RevisioningEnabled: true, BareNameSelector: "default"},
		{Name: "Node", PluralName: "nodes", Package: "example.com/revisions/apis/v1", PackageAlias: "v1", TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", URLPath: "/nodes", StorageName: "Node"},
	}
	return gen
}

func assertGeneratedMarkers(t *testing.T, path string, markers []string) {
	t.Helper()
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, marker := range markers {
		if !strings.Contains(string(generated), marker) {
			t.Errorf("%s missing %q", filepath.Base(path), marker)
		}
	}
}

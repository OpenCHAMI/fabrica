// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"strings"
	"testing"
)

func TestGeneratedHandlers_bound_every_request_body_with_stable_413_contract(t *testing.T) {
	// Given
	gen := NewGenerator(t.TempDir(), "main", "example.com/body-limit")
	gen.Version = "test"
	resource := ResourceMetadata{
		Name: "Token", PluralName: "tokens", Package: "example.com/body-limit/apis/example.io/v1",
		PackageAlias: "v1", TypeName: "*v1.Token", SpecType: "v1.TokenSpec",
		StatusType: "v1.TokenStatus", URLPath: "/tokens", StorageName: "Token",
	}
	gen.Resources = []ResourceMetadata{resource}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	var handlers bytes.Buffer
	var models bytes.Buffer
	var routes bytes.Buffer

	// When
	if err := gen.Templates["handlers"].Execute(&handlers, gen.templateData(resource, "server/handlers.go.tmpl")); err != nil {
		t.Fatal(err)
	}
	if err := gen.Templates["models"].Execute(&models, gen.globalTemplateData("server/models.go.tmpl")); err != nil {
		t.Fatal(err)
	}
	if err := gen.Templates["routes"].Execute(&routes, gen.globalTemplateData("server/routes.go.tmpl")); err != nil {
		t.Fatal(err)
	}

	// Then
	generatedHandlers := handlers.String()
	if got := strings.Count(generatedHandlers, "decodeRequestBody(w, r,"); got != 5 {
		t.Fatalf("bounded JSON decode calls=%d, want 5\n%s", got, generatedHandlers)
	}
	if got := strings.Count(generatedHandlers, "readRequestBody(w, r)"); got != 4 {
		t.Fatalf("bounded read calls=%d, want 4\n%s", got, generatedHandlers)
	}
	for _, unbounded := range []string{"json.NewDecoder(r.Body)", "io.ReadAll(r.Body)"} {
		if strings.Contains(generatedHandlers, unbounded) {
			t.Fatalf("generated handlers retain unbounded body read %q\n%s", unbounded, generatedHandlers)
		}
	}
	generatedModels := models.String()
	for _, contract := range []string{
		"httpbody.DecodeOne(w, r, destination)",
		"httpbody.ReadAll(w, r)",
		"httpbody.IsTooLarge(err)",
		"httpbody.WriteTooLarge(w)",
	} {
		if !strings.Contains(generatedModels, contract) {
			t.Errorf("generated request body contract missing %q\n%s", contract, generatedModels)
		}
	}
	generatedRoutes := routes.String()
	for _, contract := range []string{
		"const DefaultRequestBodyMaxBytes int64 = httpbody.DefaultMaxBytes",
		"func newRequestBodyLimitMiddleware(defaultLimit int64, overrides map[string]int64)",
		"httpbody.Apply(w, r, limit)",
		`case "Token":`,
	} {
		if !strings.Contains(generatedRoutes, contract) {
			t.Errorf("generated router body contract missing %q\n%s", contract, generatedRoutes)
		}
	}
	initMain, err := GetEmbeddedTemplates().ReadFile("templates/init/main.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	mainSource := string(initMain)
	limitUse := strings.Index(mainSource, "r.Use(bodyLimitMiddleware)")
	versionUse := strings.Index(mainSource, "r.Use(versioning.VersionNegotiationMiddleware")
	if limitUse < 0 || versionUse < 0 || limitUse >= versionUse {
		t.Fatalf("request body middleware must be registered before version negotiation\n%s", mainSource)
	}
	limitValidation := strings.Index(mainSource, "newRequestBodyLimitMiddleware(config.RequestBodyMaxBytes")
	storageInitialization := strings.Index(mainSource, "initializeStorage(config)")
	if limitValidation < 0 || storageInitialization < 0 || limitValidation >= storageInitialization {
		t.Fatalf("request body limit validation must precede storage initialization\n%s", mainSource)
	}
}

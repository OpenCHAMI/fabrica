// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGeneratedAPICatalog(t *testing.T) {
	for _, tt := range []struct {
		name, serverURL string
		paths           []string
	}{
		{name: "default", paths: []string{"/nodes", "/boot/configs"}},
		{name: "configured base path", serverURL: "https://api.example.com/prefix/", paths: []string{"/inventory/nodes"}},
		{name: "no resources"},
		{name: "no resources with base path", serverURL: "https://api.example.com/prefix/"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			root, err := filepath.Abs(filepath.Join("..", ".."))
			if err != nil {
				t.Fatal(err)
			}
			writeOpenAPIValidationFixture(t, dir, root)
			gen := NewGenerator(dir, "main", "example.com/openapifixture")
			if tt.serverURL != "" {
				gen.Config.OpenAPIServerURL = tt.serverURL
			}
			for i, path := range tt.paths {
				name := []string{"Node", "BootConfig"}[i]
				gen.Resources = append(gen.Resources, ResourceMetadata{
					Name: name, PluralName: name + "s", URLPath: path,
					Package: "example.com/openapifixture/apis/v1", PackageAlias: "v1",
					TypeName: "*v1." + name, SpecType: "v1." + name + "Spec", StatusType: "v1." + name + "Status",
				})
			}
			if err := gen.LoadTemplates(); err != nil {
				t.Fatal(err)
			}
			if err := gen.GenerateRoutes(); err != nil {
				t.Fatal(err)
			}

			if err := gen.GenerateModels(); err != nil {
				t.Fatal(err)
			}
			if err := gen.GenerateOpenAPI(); err != nil {
				t.Fatal(err)
			}

			// Compile generated discovery, status, and OpenAPI handlers without
			// pulling in unrelated resource handlers and storage dependencies.
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, filepath.Join(dir, "routes_generated.go"), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			var source bytes.Buffer
			source.WriteString("package main\nimport (\"encoding/json\"; \"net/http\"; \"strings\"; \"github.com/go-chi/chi/v5\")\n")
			for _, decl := range parsed.Decls {
				if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.IMPORT {
					continue
				}
				if fn, ok := decl.(*ast.FuncDecl); ok && (fn.Name.Name == "RegisterGeneratedRoutes" || fn.Name.Name == "registerResourcePrefixes") {
					continue
				}
				if err := format.Node(&source, fset, decl); err != nil {
					t.Fatal(err)
				}
				source.WriteByte('\n')
			}
			// With no resource handlers, compile the entire routes file as emitted.
			if len(gen.Resources) > 0 {
				writeConditionalFixtureFile(t, dir, "routes_generated.go", source.String())
			}
			base := strings.TrimRight(gen.Config.OpenAPIServerURL, "/")
			paths := []string{strconv.Quote(base + "/service/status")}
			for _, path := range tt.paths {
				paths = append(paths, strconv.Quote(base+path))
			}
			fixture := "package main\nconst expectedBase = " + strconv.Quote(base) + "\nvar expectedPaths = []string{" + strings.Join(paths, ",") + "}\n"
			writeConditionalFixtureFile(t, dir, "expected_test.go", fixture)
			writeConditionalFixtureFile(t, dir, "catalog_test.go", generatedCatalogBehaviorTest)
			cmd := exec.Command("go", "test", "-mod=mod", "-count=1", ".")
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("test generated API catalog: %v\n%s", err, output)
			}
		})
	}
}

const generatedCatalogBehaviorTest = `package main

import (
 "context"
 "encoding/json"
 "mime"
 "strings"
 "net/http"
 "net/http/httptest"
 "testing"

 "github.com/getkin/kin-openapi/openapi3"
 "github.com/go-chi/chi/v5"
)

func TestCatalogDiscovery(t *testing.T) {
 router := chi.NewRouter()
 router.Group(func(public chi.Router) {
  RegisterGeneratedPublicRoutes(public)
  public.Get("/openapi.json", ServeOpenAPISpec)
  public.Get("/docs", ServeSwaggerUI)
 })
 // Model a reverse proxy stripping the configured deployment prefix.
 fetch := func(target string) *httptest.ResponseRecorder {
  t.Helper()
  if !strings.HasPrefix(target, expectedBase+"/") { t.Fatalf("unexpected public URL: %s", target) }
  response := httptest.NewRecorder()
  router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, strings.TrimPrefix(target, expectedBase), nil))
  return response
 }
 router.Group(func(protected chi.Router) {
  protected.Use(func(next http.Handler) http.Handler {
   return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
  })
  protected.Get("/private", func(http.ResponseWriter, *http.Request) {})
 })
 private := httptest.NewRecorder()
 router.ServeHTTP(private, httptest.NewRequest(http.MethodGet, "/private", nil))
 if private.Code != http.StatusUnauthorized { t.Fatalf("protected route = %d", private.Code) }

 for _, method := range []string{http.MethodGet, http.MethodHead} {
  t.Run(method, func(t *testing.T) {
   req := httptest.NewRequest(method, "https://untrusted.example/.well-known/api-catalog", nil)
   req.Header.Set("Forwarded", "host=proxy.invalid;proto=http")
   req.Header.Set("X-Forwarded-Host", "proxy.invalid")
   response := httptest.NewRecorder()
   router.ServeHTTP(response, req)
   if response.Code != http.StatusOK { t.Fatalf("status = %d: %s", response.Code, response.Body.String()) }
   mediaType, params, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
   if err != nil || mediaType != "application/linkset+json" || params["profile"] != "https://www.rfc-editor.org/info/rfc9727" {
    t.Fatalf("Content-Type = %q (%v)", response.Header().Get("Content-Type"), err)
   }
   wantLink := "<" + expectedBase + "/.well-known/api-catalog>; rel=\"api-catalog\""
   if response.Header().Get("Link") != wantLink { t.Fatalf("Link = %q, want %q", response.Header().Get("Link"), wantLink) }
   if method == http.MethodHead {
    if response.Body.Len() != 0 { t.Fatalf("HEAD body = %s", response.Body.String()) }
    return
   }
   var doc struct { Linkset []map[string]json.RawMessage }
   if err := json.Unmarshal(response.Body.Bytes(), &doc); err != nil { t.Fatal(err) }
   if len(doc.Linkset) != len(expectedPaths)+1 { t.Fatalf("contexts = %d", len(doc.Linkset)) }
   var anchor string
   if err := json.Unmarshal(doc.Linkset[0]["anchor"], &anchor); err != nil { t.Fatal(err) }
   if anchor != expectedBase+"/.well-known/api-catalog" { t.Fatalf("catalog anchor = %q", anchor) }
   type link struct { Href, Type string }
   var items []link
   if raw := doc.Linkset[0]["item"]; raw != nil {
    if err := json.Unmarshal(raw, &items); err != nil { t.Fatal(err) }
   }
   if len(items) != len(expectedPaths) { t.Fatalf("items = %d", len(items)) }
   for i, path := range expectedPaths {
    if items[i].Href != path { t.Errorf("item = %q, want %q", items[i].Href, path) }
    entry := doc.Linkset[i+1]
    if err := json.Unmarshal(entry["anchor"], &anchor); err != nil { t.Fatal(err) }
    if anchor != path { t.Errorf("API anchor = %q, want %q", anchor, path) }
    for relation, want := range map[string]link{
     "service-desc": {Href: expectedBase+"/openapi.json", Type: "application/json"},
     "service-doc": {Href: expectedBase+"/docs", Type: "text/html"},
     "status": {Href: expectedBase+"/service/status", Type: "application/json"},
    } {
     var links []link
     if err := json.Unmarshal(entry[relation], &links); err != nil { t.Fatal(err) }
     if len(links) != 1 || links[0] != want { t.Fatalf("%s = %+v, want %+v", relation, links, want) }
     linked := fetch(links[0].Href)
     if linked.Code != http.StatusOK { t.Fatalf("%s link returned %d", relation, linked.Code) }
     if relation == "service-desc" {
      spec, err := openapi3.NewLoader().LoadFromData(linked.Body.Bytes())
      if err != nil { t.Fatal(err) }
      if err := spec.Validate(context.Background()); err != nil { t.Fatal(err) }
      endpoint := strings.TrimPrefix(path, expectedBase)
      if spec.Paths.Value(endpoint) == nil { t.Fatalf("OpenAPI does not describe %s", endpoint) }
      if i == 0 {
       status := fetch(items[i].Href)
       if status.Code != http.StatusOK { t.Fatalf("service item returned %d", status.Code) }
      }
     }
    }
   }
  })
 }
}
`

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRevisionWorkflowGeneratedAPI(t *testing.T) {
	dir := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	writeRevisionFixture(t, dir, root)

	gen := NewGenerator(dir, "main", "example.com/revisionfixture")
	gen.Config.VersioningEnabled = true
	gen.Config.ConditionalEnabled = false
	gen.Resources = []ResourceMetadata{{
		Name: "Node", PluralName: "nodes", Package: "example.com/revisionfixture/apis/v1", PackageAlias: "v1",
		TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", URLPath: "/nodes",
		StorageName: "Node", RevisioningEnabled: true, BareNameSelector: "default",
	}}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	for name, generate := range map[string]func() error{
		"models": gen.GenerateModels, "handlers": gen.GenerateHandlers, "routes": gen.GenerateRoutes,
	} {
		if err := generate(); err != nil {
			t.Fatalf("generate %s: %v", name, err)
		}
	}

	var storage bytes.Buffer
	if err := gen.Templates["storage"].Execute(&storage, gen.globalTemplateData("storage/file.go.tmpl")); err != nil {
		t.Fatalf("render storage: %v", err)
	}
	formatted, err := format.Source(storage.Bytes())
	if err != nil {
		t.Fatalf("format generated storage: %v", err)
	}
	storageDir := filepath.Join(dir, "internal", "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatalf("create storage directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "storage_generated.go"), formatted, 0o644); err != nil {
		t.Fatalf("write generated storage: %v", err)
	}

	for _, args := range [][]string{{"mod", "tidy"}, {"test", "-count=1", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.6")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("execute generated revision API go %v: %v\n%s", args, err, output)
		}
	}
}

func writeRevisionFixture(t *testing.T, dir, root string) {
	t.Helper()
	files := map[string]string{
		"go.mod": fmt.Sprintf("module example.com/revisionfixture\n\ngo 1.26.6\n\nrequire (\n github.com/go-chi/chi/v5 v5.2.3\n github.com/openchami/fabrica v0.0.0\n)\n\nreplace github.com/openchami/fabrica => %s\n", root),
		"apis/v1/types.go": `package v1
import "github.com/openchami/fabrica/pkg/resource"
type NodeSpec struct { Value string ` + "`json:\"value\" validate:\"required\"`" + ` }
type NodeStatus struct { Phase string ` + "`json:\"phase,omitempty\" validate:\"omitempty,oneof=Pending Ready\"`" + ` }
type Node struct { APIVersion string ` + "`json:\"apiVersion\"`" + `; Kind string ` + "`json:\"kind\"`" + `; Metadata resource.Metadata ` + "`json:\"metadata\"`" + `; Spec NodeSpec ` + "`json:\"spec\"`" + `; Status NodeStatus ` + "`json:\"status\"`" + ` }
`,
		"revision_workflow_test.go": `package main
import (
 "bytes"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"
 "github.com/go-chi/chi/v5"
 "github.com/openchami/fabrica/pkg/revision"
 "example.com/revisionfixture/internal/storage"
 v1 "example.com/revisionfixture/apis/v1"
)
func request(t *testing.T, router http.Handler, method, path, body, ifMatch string) *httptest.ResponseRecorder { t.Helper(); req := httptest.NewRequest(method, path, bytes.NewBufferString(body)); req.Header.Set("Content-Type", "application/json"); if ifMatch != "" { req.Header.Set("If-Match", ifMatch) }; response := httptest.NewRecorder(); router.ServeHTTP(response, req); return response }
func decodeRecord(t *testing.T, response *httptest.ResponseRecorder) revision.Record[*v1.Node] { t.Helper(); var record revision.Record[*v1.Node]; if err := json.Unmarshal(response.Body.Bytes(), &record); err != nil { t.Fatal(err) }; return record }
func TestWorkflow(t *testing.T) {
 if err := storage.InitFileBackend(t.TempDir()); err != nil { t.Fatal(err) }
 if err := registerResourcePrefixes(); err != nil { t.Fatal(err) }
 router := chi.NewRouter(); RegisterGeneratedRoutes(router)
 firstBody := "{\"metadata\":{\"name\":\"production\"},\"spec\":{\"value\":\"one\"}}"
 invalidSeries := request(t, router, http.MethodPost, "/nodes/", "{\"metadata\":{\"name\":\"invalid\"},\"spec\":{}}", ""); if invalidSeries.Code != http.StatusBadRequest { t.Fatalf("invalid series = %d: %s", invalidSeries.Code, invalidSeries.Body.String()) }
 created := request(t, router, http.MethodPost, "/nodes/", firstBody, ""); if created.Code != http.StatusCreated { t.Fatalf("create = %d: %s", created.Code, created.Body.String()) }; first := decodeRecord(t, created)
 duplicate := request(t, router, http.MethodPost, "/nodes/", firstBody, ""); if duplicate.Code != http.StatusConflict { t.Fatalf("duplicate = %d", duplicate.Code) }
 invalidStatus := request(t, router, http.MethodPut, "/nodes/production/status", "{\"phase\":\"Broken\"}", ""); if invalidStatus.Code != http.StatusBadRequest { t.Fatalf("invalid status = %d: %s", invalidStatus.Code, invalidStatus.Body.String()) }
 status := request(t, router, http.MethodPut, "/nodes/production/status", "{\"phase\":\"Ready\"}", ""); if status.Code != http.StatusOK || !bytes.Contains(status.Body.Bytes(), []byte("Ready")) { t.Fatalf("status = %d: %s", status.Code, status.Body.String()) }
 invalidStatusPatch := request(t, router, http.MethodPatch, "/nodes/production/status", "{\"phase\":\"Broken\"}", ""); if invalidStatusPatch.Code != http.StatusBadRequest { t.Fatalf("invalid status patch = %d: %s", invalidStatusPatch.Code, invalidStatusPatch.Body.String()) }
 secondBody := "{\"metadata\":{},\"spec\":{\"value\":\"two\"}}"
 invalidEnsure := request(t, router, http.MethodPut, "/nodes/production/revisions/invalid", "{\"metadata\":{},\"spec\":{}}", ""); if invalidEnsure.Code != http.StatusBadRequest { t.Fatalf("invalid ensure = %d: %s", invalidEnsure.Code, invalidEnsure.Body.String()) }
 ensured := request(t, router, http.MethodPut, "/nodes/production/revisions/release-b", secondBody, ""); if ensured.Code != http.StatusCreated { t.Fatalf("ensure = %d: %s", ensured.Code, ensured.Body.String()) }; second := decodeRecord(t, ensured); etag := ensured.Header().Get("ETag")
 retry := request(t, router, http.MethodPut, "/nodes/production/revisions/release-b", secondBody, ""); if retry.Code != http.StatusOK || decodeRecord(t, retry).Metadata.UID != second.Metadata.UID { t.Fatalf("retry = %d", retry.Code) }
 seriesRead := request(t, router, http.MethodGet, "/nodes/production/", "", ""); if seriesRead.Code != http.StatusOK || seriesRead.Header().Get("ETag") == "" { t.Fatalf("series read = %d ETag %q", seriesRead.Code, seriesRead.Header().Get("ETag")) }
 revisionList := request(t, router, http.MethodGet, "/nodes/production/revisions", "", ""); if revisionList.Code != http.StatusOK || revisionList.Header().Get("ETag") == "" { t.Fatalf("revision list = %d ETag %q", revisionList.Code, revisionList.Header().Get("ETag")) }
 revisionGet := request(t, router, http.MethodGet, "/nodes/production/revisions/by-uid/"+second.Metadata.UID, "", ""); if revisionGet.Code != http.StatusOK || revisionGet.Header().Get("ETag") == "" { t.Fatalf("revision get = %d ETag %q", revisionGet.Code, revisionGet.Header().Get("ETag")) }
 conflict := request(t, router, http.MethodPut, "/nodes/production/revisions/release-b", "{\"metadata\":{},\"spec\":{\"value\":\"changed\"}}", ""); if conflict.Code != http.StatusConflict { t.Fatalf("conflict = %d", conflict.Code) }
 var conflictBody revision.ConflictResponse; if err := json.Unmarshal(conflict.Body.Bytes(), &conflictBody); err != nil { t.Fatal(err) }; if conflictBody.ExistingRevisionUID != second.Metadata.UID || conflictBody.ExistingDigest == "" || conflictBody.RequestedDigest == "" { t.Fatalf("conflict body = %#v", conflictBody) }
 defaultResolved := request(t, router, http.MethodGet, "/nodes/production/resolve", "", ""); if defaultResolved.Code != http.StatusOK || defaultResolved.Header().Get("ETag") == "" || !bytes.Contains(defaultResolved.Body.Bytes(), []byte(first.Metadata.UID)) { t.Fatalf("default resolve = %d ETag %q: %s", defaultResolved.Code, defaultResolved.Header().Get("ETag"), defaultResolved.Body.String()) }
 latestResolved := request(t, router, http.MethodGet, "/nodes/production/resolve?selector=latest", "", ""); if latestResolved.Code != http.StatusOK || latestResolved.Header().Get("ETag") == "" || !bytes.Contains(latestResolved.Body.Bytes(), []byte(second.Metadata.UID)) { t.Fatalf("latest resolve = %d ETag %q: %s", latestResolved.Code, latestResolved.Header().Get("ETag"), latestResolved.Body.String()) }
  promotionBody := "{\"revisionUid\":\"" + second.Metadata.UID + "\"}"
	for _, invalidPromotionBody := range []string{"{}", "{\"revisionUid\":\"\"}"} { invalidPromotion := request(t, router, http.MethodPut, "/nodes/production/default", invalidPromotionBody, etag); if invalidPromotion.Code != http.StatusBadRequest { t.Fatalf("invalid promotion body %s = %d: %s", invalidPromotionBody, invalidPromotion.Code, invalidPromotion.Body.String()) } }
  promoted := request(t, router, http.MethodPut, "/nodes/production/default", promotionBody, etag); if promoted.Code != http.StatusOK { t.Fatalf("promote = %d: %s", promoted.Code, promoted.Body.String()) }
	stale := request(t, router, http.MethodPut, "/nodes/production/default", "{\"revisionUid\":\"" + first.Metadata.UID + "\"}", etag); if stale.Code != http.StatusPreconditionFailed { t.Fatalf("stale = %d", stale.Code) }
	var staleBody revision.PreconditionResponse; if err := json.Unmarshal(stale.Body.Bytes(), &staleBody); err != nil { t.Fatal(err) }; if staleBody.CurrentETag == "" { t.Fatalf("stale body = %#v", staleBody) }
	retired := request(t, router, http.MethodDelete, "/nodes/production/revisions/by-uid/"+first.Metadata.UID, "", ""); if retired.Code != http.StatusOK || !bytes.Contains(retired.Body.Bytes(), []byte("retiredAt")) { t.Fatalf("retire = %d: %s", retired.Code, retired.Body.String()) }
	retiredGet := request(t, router, http.MethodGet, "/nodes/production/revisions/by-uid/"+first.Metadata.UID, "", ""); if retiredGet.Code != http.StatusGone { t.Fatalf("retired get = %d: %s", retiredGet.Code, retiredGet.Body.String()) }
 invalidRevision := request(t, router, http.MethodPost, "/nodes/production/revisions", "{\"metadata\":{},\"spec\":{}}", ""); if invalidRevision.Code != http.StatusBadRequest { t.Fatalf("invalid revision = %d: %s", invalidRevision.Code, invalidRevision.Body.String()) }
	third := request(t, router, http.MethodPost, "/nodes/production/revisions", "{\"metadata\":{},\"spec\":{\"value\":\"three\"}}", ""); if third.Code != http.StatusCreated || decodeRecord(t, third).Metadata.Number != 3 { t.Fatalf("third = %d: %s", third.Code, third.Body.String()) }
}
`,
	}
	for relative, content := range files {
		path := filepath.Join(dir, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", relative, err)
		}
	}
}

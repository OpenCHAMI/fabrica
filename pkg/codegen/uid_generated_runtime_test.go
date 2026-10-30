// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUIDDefaultGeneratedWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated UID workflow integration test in short mode")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []string{"file", "ent"} {
		t.Run(backend, func(t *testing.T) {
			dir := t.TempDir()
			writeRevisionFixture(t, dir, root)
			typesPath := filepath.Join(dir, "apis", "v1", "types.go")
			types, err := os.ReadFile(typesPath)
			if err != nil {
				t.Fatal(err)
			}
			types = append(types, []byte("\nfunc (n *Node) GetUID() string { return n.Metadata.UID }\n")...)
			if err := os.WriteFile(typesPath, types, 0o644); err != nil {
				t.Fatal(err)
			}
			if backend == "ent" {
				modulePath := filepath.Join(dir, "go.mod")
				module, err := os.ReadFile(modulePath)
				if err != nil {
					t.Fatal(err)
				}
				module = append(module, []byte("\nrequire (\n entgo.io/ent v0.14.5\n github.com/mattn/go-sqlite3 v1.14.32\n)\n")...)
				if err := os.WriteFile(modulePath, module, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Error(err)
				}
			})
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			gen := NewGenerator(dir, "main", "example.com/revisionfixture")
			gen.Config.VersioningEnabled = true
			gen.Config.ConditionalEnabled = false
			gen.SetStorageType(backend)
			gen.DBDriver = "sqlite3"
			gen.Resources = []ResourceMetadata{{
				Name: "Node", PluralName: "nodes", Package: "example.com/revisionfixture/apis/v1", PackageAlias: "v1",
				TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", URLPath: "/nodes", StorageName: "Node",
			}}
			if err := gen.LoadTemplates(); err != nil {
				t.Fatal(err)
			}
			if backend == "ent" {
				for _, generate := range []func() error{gen.GenerateEntSchemas, gen.GenerateEntAdapter, gen.GenerateEntHelpers} {
					if err := generate(); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, generate := range []func() error{gen.GenerateModels, gen.GenerateHandlers, gen.GenerateRoutes, gen.GenerateOpenAPI, gen.GenerateStorage} {
				if err := generate(); err != nil {
					t.Fatal(err)
				}
			}
			imports := ""
			init := `if err := storage.InitFileBackend(t.TempDir()); err != nil { t.Fatal(err) }`
			if backend == "ent" {
				imports = `"database/sql"
    "path/filepath"
    "example.com/revisionfixture/internal/storage/ent"
    dialectsql "entgo.io/ent/dialect/sql"
    _ "github.com/mattn/go-sqlite3"`
				init = `db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "uids.db")+"?_fk=1")
    if err != nil { t.Fatal(err) }
    client := ent.NewClient(ent.Driver(dialectsql.OpenDB("sqlite3", db)))
    t.Cleanup(func() { _ = client.Close() })
    if err := client.Schema.Create(context.Background()); err != nil { t.Fatal(err) }
    storage.SetEntClient(client)`
			}
			source := strings.ReplaceAll(uidWorkflowSource, "BACKEND_IMPORTS", imports)
			source = strings.ReplaceAll(source, "INIT_BACKEND", init)
			if err := os.WriteFile(filepath.Join(dir, "revision_workflow_test.go"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			commands := [][]string{{"mod", "tidy"}, {"test", "-count=1", "./..."}}
			if backend == "ent" {
				commands = append([][]string{{"run", "-mod=mod", "entgo.io/ent/cmd/ent", "generate", "./internal/storage/ent/schema"}}, commands...)
			}
			for _, args := range commands {
				cmd := exec.Command("go", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.10")
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("generated UID workflow go %v: %v\n%s", args, err, output)
				}
			}
		})
	}
}

const uidWorkflowSource = `package main
import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "strings"
    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/openchami/fabrica/pkg/resource"
    "github.com/openchami/fabrica/pkg/uid"
    "example.com/revisionfixture/internal/storage"
    v1 "example.com/revisionfixture/apis/v1"
    BACKEND_IMPORTS
)
func TestUIDWorkflow(t *testing.T) {
    INIT_BACKEND
    if err := registerResourcePrefixes(); err != nil { t.Fatal(err) }
    router := chi.NewRouter(); RegisterGeneratedRoutes(router)
    request := func(method, path, body string, code int) *v1.Node {
        t.Helper()
        req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
        req.Header.Set("Content-Type", "application/json")
        result := httptest.NewRecorder(); router.ServeHTTP(result, req)
        if result.Code != code { t.Fatalf("%s %s = %d: %s", method, path, result.Code, result.Body.String()) }
        var node v1.Node
        if err := json.Unmarshal(result.Body.Bytes(), &node); err != nil { t.Fatal(err) }
        return &node
    }
    created := request(http.MethodPost, "/nodes/", "{\"metadata\":{\"name\":\"new-node\"},\"spec\":{\"value\":\"one\"}}", http.StatusCreated)
    parsed, err := (uid.Codec{}).Parse(created.Metadata.UID)
    if err != nil || parsed.Format != uid.FormatTypeID || parsed.Prefix != "node" { t.Fatalf("created UID = %+v, %v", parsed, err) }
    id := uuid.MustParse(parsed.UUID)
    if id.Version() != 7 || id.Variant() != uuid.RFC4122 { t.Fatalf("new payload = %s", id) }
    // Seed the exact historical identity rather than relying on a create endpoint
    // to accept client-supplied IDs. No migration or rewrite is performed.
    legacy := &v1.Node{APIVersion:"v1", Kind:"Node", Metadata:resource.Metadata{Name:"old-node", UID:"node-1a2b3c4d"}, Spec:v1.NodeSpec{Value:"old"}, Status:v1.NodeStatus{Phase:"Ready"}}
    if err := storage.SaveNode(context.Background(), legacy); err != nil { t.Fatal(err) }
    for _, value := range []string{created.Metadata.UID, legacy.Metadata.UID} {
        loaded := request(http.MethodGet, "/nodes/"+value, "", http.StatusOK)
        if loaded.Metadata.UID != value { t.Fatal("read rewrote UID") }
        updated := request(http.MethodPut, "/nodes/"+value, "{\"metadata\":{\"uid\":\"replacement\"},\"spec\":{\"value\":\"two\"}}", http.StatusOK)
        if updated.Metadata.UID != value || updated.Spec.Value != "two" { t.Fatalf("update changed UID: %+v", updated) }
        stored, err := storage.LoadNode(context.Background(), value)
        if err != nil || stored.Metadata.UID != value { t.Fatalf("storage identity = %+v, %v", stored, err) }
        if value == legacy.Metadata.UID && stored.Status.Phase != "Ready" { t.Fatal("spec update changed legacy status") }
    }
    spec := GenerateOpenAPISpec()
    if err := spec.Validate(context.Background()); err != nil { t.Fatal(err) }
    path := spec.Paths.Find("/nodes/{uid}")
    found := false
    for _, parameter := range path.Parameters {
        if parameter.Value.Name == "uid" {
            found = true
            if !strings.Contains(parameter.Value.Description, "legacy") { t.Fatal("UID description omits legacy reads") }
            example, ok := parameter.Value.Schema.Value.Example.(string)
            if !ok || !strings.HasPrefix(example, "node_") || !resource.IsValidUID(example) { t.Fatalf("invalid UID example: %#v", parameter.Value.Schema.Value.Example) }
        }
    }
    if !found { t.Fatal("OpenAPI UID parameter missing") }
}
`

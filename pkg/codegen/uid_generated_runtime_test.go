// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	_ "embed"
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
	for _, tc := range []struct {
		name, backend string
		conditional   bool
	}{
		{name: "file", backend: "file"},
		{name: "ent", backend: "ent"},
		{name: "file_conditional", backend: "file", conditional: true},
		{name: "ent_conditional", backend: "ent", conditional: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := tc.backend
			dir := t.TempDir()
			writeRevisionFixture(t, dir, root)
			typesPath := filepath.Join(dir, "apis", "v1", "types.go")
			types, err := os.ReadFile(typesPath)
			if err != nil {
				t.Fatal(err)
			}
			types = []byte(strings.Replace(string(types), "type NodeSpec struct {", "type NodeSpec struct { ReferenceUID string `json:\"referenceUID,omitempty\" yaml:\"referenceUID,omitempty\"`;", 1))
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
			gen.Config.ConditionalEnabled = tc.conditional
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
			if tc.conditional {
				if err := gen.GenerateMiddleware(); err != nil {
					t.Fatal(err)
				}
			}
			clientGen := NewGenerator(filepath.Join(dir, "pkg", "client"), "client", "example.com/revisionfixture")
			if err := os.MkdirAll(clientGen.OutputDir, 0o755); err != nil {
				t.Fatal(err)
			}
			clientGen.Config.VersioningEnabled = true
			clientGen.Resources = gen.Resources
			if err := clientGen.LoadTemplates(); err != nil {
				t.Fatal(err)
			}
			for _, generate := range []func() error{clientGen.GenerateClient, clientGen.GenerateClientModels, clientGen.GenerateClientCmd, clientGen.GenerateClientVersion} {
				if err := generate(); err != nil {
					t.Fatal(err)
				}
			}
			imports := `"os"`
			init := `supportsDeleteFailure := false
    setDeleteFailure := func(bool) {}
    dataDir := t.TempDir()
    reopen := func() {
        if storage.Backend != nil { if err := storage.Backend.Close(); err != nil { t.Fatal(err) } }
        if err := storage.InitFileBackend(dataDir); err != nil { t.Fatal(err) }
    }
    reopen()
    t.Cleanup(func() { _ = storage.Backend.Close() })
    verifyKeys := func(ids []string) {
        entries, err := os.ReadDir(filepath.Join(dataDir, "nodes"))
        if err != nil { t.Fatal(err) }
        if len(entries) != len(ids) { t.Fatalf("physical file count = %d, want %d", len(entries), len(ids)) }
        want := make(map[string]bool)
        for _, id := range ids { want[id+".json"] = true }
        for _, entry := range entries { if !want[entry.Name()] { t.Fatalf("UID filename changed: %s", entry.Name()) } }
    }`
			if backend == "ent" {
				imports = `"database/sql"
    "errors"
    "sync/atomic"
    entgo "entgo.io/ent"
    "example.com/revisionfixture/internal/storage/ent"
    dialectsql "entgo.io/ent/dialect/sql"
    _ "github.com/mattn/go-sqlite3"`
				init = `supportsDeleteFailure := true
    failDelete := &atomic.Bool{}
    setDeleteFailure := func(enabled bool) { failDelete.Store(enabled) }
    dbPath := filepath.Join(t.TempDir(), "uids.db")
    var dbClient *ent.Client
    reopen := func() {
        if dbClient != nil { if err := dbClient.Close(); err != nil { t.Fatal(err) } }
        db, err := sql.Open("sqlite3", "file:"+dbPath+"?_fk=1")
        if err != nil { t.Fatal(err) }
        dbClient = ent.NewClient(ent.Driver(dialectsql.OpenDB("sqlite3", db)))
        if err := dbClient.Schema.Create(context.Background()); err != nil { t.Fatal(err) }
        dbClient.Resource.Use(func(next entgo.Mutator) entgo.Mutator {
            return entgo.MutateFunc(func(ctx context.Context, mutation entgo.Mutation) (entgo.Value, error) {
                if failDelete.Load() && mutation.Op().Is(entgo.OpDelete | entgo.OpDeleteOne) { return nil, errors.New("forced resource delete failure") }
                return next.Mutate(ctx, mutation)
            })
        })
        storage.SetEntClient(dbClient)
    }
    reopen()
    t.Cleanup(func() { _ = dbClient.Close() })
    physicalIDs := make(map[string]int)
    verifyKeys := func(ids []string) {
        if len(ids) == 0 {
            for _, count := range []func(context.Context) (int, error){dbClient.Label.Query().Count, dbClient.Annotation.Query().Count} {
                n, err := count(context.Background()); if err != nil || n != 0 { t.Fatalf("orphaned metadata rows = %d, %v", n, err) }
            }
        }
        rows, err := dbClient.Resource.Query().All(context.Background())
        if err != nil { t.Fatal(err) }
        if len(rows) != len(ids) { t.Fatalf("physical row count = %d, want %d", len(rows), len(ids)) }
        want := make(map[string]bool)
        for _, id := range ids { want[id] = true }
        for _, row := range rows {
            if !want[row.UID] { t.Fatalf("stored UID changed: %s", row.UID) }
            if previous, ok := physicalIDs[row.UID]; ok && previous != row.ID { t.Fatalf("row recreated for UID %s", row.UID) }
            physicalIDs[row.UID] = row.ID
        }
    }`
			}
			source := strings.ReplaceAll(uidWorkflowSource, "/*BACKEND_IMPORTS*/", imports)
			source = strings.ReplaceAll(source, "/*INIT_BACKEND*/", init)
			if tc.conditional {
				source = strings.ReplaceAll(source, "conditionalRequestsEnabled := false", "conditionalRequestsEnabled := true")
			}
			if err := os.WriteFile(filepath.Join(dir, "revision_workflow_test.go"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			commands := [][]string{{"mod", "tidy"}, {"build", "-o", "uid-client", "./cmd/client"}, {"test", "-race", "-count=1", "./..."}}
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

//go:embed testdata/uid_migration_workflow_test.go.fixture
var uidWorkflowSource string

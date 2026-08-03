// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/codegen/testfixtures"
)

// Coverage for the two paths the dedicated-schema tests do not touch:
//
//   - the generic Ent schema trio (resource, label, annotation), which is what
//     every service using storage=ent gets whether or not it declares
//     annotations;
//   - the storage adapter, queries and transactions, which are the code that
//     actually moves data between Fabrica resources and Ent entities.
//
// The adapter is the more valuable of the two to cover: it imports the Ent
// client that entc generates from our schemas, so nothing short of running the
// whole chain can tell you it compiles.

// fabricaRoot returns the repository root, so a generated module can replace
// github.com/openchami/fabrica with the working tree under test.
func fabricaRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve fabrica root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected fabrica go.mod at %s: %v", root, err)
	}

	return root
}

// generateProject runs the generator into a fresh directory and returns it.
// withAdapter also emits the adapter, query helpers and storage layer.
func generateProject(t *testing.T, modulePath string, withAdapter bool) string {
	t.Helper()

	projectDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	gen := NewGenerator(projectDir, "main", modulePath)
	gen.StorageType = "ent"
	gen.DBDriver = "postgres"

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if err := gen.RegisterResource(&testfixtures.Widget{}); err != nil {
		t.Fatalf("RegisterResource: %v", err)
	}
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas: %v", err)
	}

	if withAdapter {
		if err := gen.GenerateEntAdapter(); err != nil {
			t.Fatalf("GenerateEntAdapter: %v", err)
		}
		if err := gen.GenerateEntHelpers(); err != nil {
			t.Fatalf("GenerateEntHelpers: %v", err)
		}
		if err := gen.GenerateStorage(); err != nil {
			t.Fatalf("GenerateStorage: %v", err)
		}
	}

	return projectDir
}

// prepareModule writes a go.mod for the generated project and resolves deps.
// Skips the test when dependencies cannot be fetched.
func prepareModule(t *testing.T, dir, modulePath string, needAtlas bool) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping generated-code compile test in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain available")
	}

	gomod := "module " + modulePath + "\n\ngo 1.24\n\n" +
		"require github.com/openchami/fabrica v0.4.9\n\n" +
		"replace github.com/openchami/fabrica => " + fabricaRoot(t) + "\n"
	writeFile(t, filepath.Join(dir, "go.mod"), gomod)

	gets := [][]string{
		{"get", "entgo.io/ent@" + entVersion},
		{"get", "golang.org/x/crypto"},
	}
	if needAtlas {
		gets = append(gets,
			[]string{"get", "ariga.io/atlas"},
			[]string{"get", "golang.org/x/tools"},
			[]string{"get", "modernc.org/sqlite"},
		)
	}
	for _, args := range gets {
		if out, err := runIn(dir, "go", args...); err != nil {
			t.Skipf("cannot resolve dependencies (%v): %s", err, out)
		}
	}
	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed (%v): %s", err, out)
	}
}

// runEntCodegen runs Ent's own generator over the project's schema package.
func runEntCodegen(t *testing.T, dir, modulePath string) {
	t.Helper()

	prog := `package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	if err := entc.Generate("../../internal/storage/ent/schema",
		&gen.Config{Target: "../../internal/storage/ent", Package: "` + modulePath + `/internal/storage/ent"}); err != nil {
		log.Fatal(err)
	}
}
`
	writeFile(t, filepath.Join(dir, "cmd", "entgen", "main.go"), prog)

	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed (%v): %s", err, out)
	}
	if out, err := runIn(filepath.Join(dir, "cmd", "entgen"), "go", "run", "."); err != nil {
		t.Fatalf("Ent rejected the generated schemas:\n%s", out)
	}
}

// TestGenericSchemasCompile builds the generic resource/label/annotation trio.
// Every storage=ent project gets these, annotations or not.
func TestGenericSchemasCompile(t *testing.T) {
	const modulePath = "generictest"

	dir := generateProject(t, modulePath, false)
	prepareModule(t, dir, modulePath, false)

	for _, name := range []string{"resource.go", "label.go", "annotation.go"} {
		path := filepath.Join(dir, "internal", "storage", "ent", "schema", name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generic schema %s: %v", name, err)
		}
	}

	if out, err := runIn(dir, "go", "build", "./internal/..."); err != nil {
		t.Errorf("generic schemas do not compile:\n%s", out)
	}
}

// genericCRUD exercises the generic resource table plus both edges.
const genericCRUD = `package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"generictest/internal/storage/ent"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "file:g?mode=memory&cache=shared&_fk=1")
	if err != nil {
		log.Fatal(err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	defer client.Close()

	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("MIGRATE FAILED: %v", err)
	}
	fmt.Println("migrate ok")

	spec := json.RawMessage(` + "`" + `{"name":"widget-1","size":3}` + "`" + `)
	status := json.RawMessage(` + "`" + `{"phase":"Ready"}` + "`" + `)

	// Label and Annotation declare a required back-edge to Resource, so the
	// resource has to exist first.
	r, err := client.Resource.Create().
		SetUID("uid-1").SetName("widget-1").SetAPIVersion("v1").SetKind("Widget").
		SetResourceType("widgets").SetSpec(spec).SetStatus(status).
		SetResourceVersion("1").SetNamespace("default").Save(ctx)
	if err != nil {
		log.Fatalf("CREATE FAILED: %v", err)
	}
	if _, err := client.Label.Create().SetKey("env").SetValue("test").SetResource(r).Save(ctx); err != nil {
		log.Fatalf("LABEL FAILED: %v", err)
	}
	if _, err := client.Annotation.Create().SetKey("owner").SetValue("team-a").SetResource(r).Save(ctx); err != nil {
		log.Fatalf("ANNOTATION FAILED: %v", err)
	}

	got, err := client.Resource.Get(ctx, r.ID)
	if err != nil {
		log.Fatalf("READ FAILED: %v", err)
	}

	fail := 0
	check := func(n string, ok bool, d string) {
		s := "ok"
		if !ok {
			s = "FAIL"
			fail++
		}
		fmt.Printf("%s %s %s\n", s, n, d)
	}

	var backSpec map[string]interface{}
	if err := json.Unmarshal(got.Spec, &backSpec); err != nil {
		log.Fatalf("spec unmarshal: %v", err)
	}
	check("spec JSON", backSpec["name"] == "widget-1" && backSpec["size"].(float64) == 3, fmt.Sprint(backSpec))

	var backStatus map[string]interface{}
	if err := json.Unmarshal(got.Status, &backStatus); err != nil {
		log.Fatalf("status unmarshal: %v", err)
	}
	check("status JSON", backStatus["phase"] == "Ready", fmt.Sprint(backStatus))

	labels, err := got.QueryLabels().All(ctx)
	if err != nil {
		log.Fatalf("QUERY LABELS FAILED: %v", err)
	}
	check("labels edge", len(labels) == 1 && labels[0].Key == "env", fmt.Sprint(len(labels)))

	anns, err := got.QueryAnnotations().All(ctx)
	if err != nil {
		log.Fatalf("QUERY ANNOTATIONS FAILED: %v", err)
	}
	check("annotations edge", len(anns) == 1 && anns[0].Key == "owner", fmt.Sprint(len(anns)))

	if fail > 0 {
		log.Fatalf("%d check(s) failed", fail)
	}
	fmt.Println("all round-trips ok")
}
`

// TestGenericSchemaRoundTripsThroughEnt migrates the generic tables and proves
// the JSON spec/status columns and both edges work against a real database.
func TestGenericSchemaRoundTripsThroughEnt(t *testing.T) {
	const modulePath = "generictest"

	dir := generateProject(t, modulePath, false)
	prepareModule(t, dir, modulePath, true)
	runEntCodegen(t, dir, modulePath)

	writeFile(t, filepath.Join(dir, "cmd", "crud", "main.go"), genericCRUD)
	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed (%v): %s", err, out)
	}

	out, err := runIn(filepath.Join(dir, "cmd", "crud"), "go", "run", ".")
	if err != nil {
		t.Fatalf("generic round-trip failed:\n%s", out)
	}

	t.Logf("generic round-trip:\n%s", out)

	if !strings.Contains(out, "migrate ok") {
		t.Error("generic migration did not report success")
	}
	if !strings.Contains(out, "all round-trips ok") || strings.Contains(out, "FAIL ") {
		t.Errorf("generic round-trip incomplete:\n%s", out)
	}
}

// TestGeneratedStorageLayerCompiles is the adapter's coverage, and the widest
// chain in the suite: fabrica generates schemas AND the adapter, entc generates
// the Ent client from those schemas, and then the adapter is compiled against
// that client. The adapter imports the generated package, so this is the only
// way to know it builds.
func TestGeneratedStorageLayerCompiles(t *testing.T) {
	const modulePath = "adaptertest"

	dir := generateProject(t, modulePath, true)
	prepareModule(t, dir, modulePath, true)

	for _, name := range []string{
		"ent_adapter.go",
		"ent_queries_generated.go",
		"ent_transactions_generated.go",
		"storage_generated.go",
	} {
		if _, err := os.Stat(filepath.Join(dir, "internal", "storage", name)); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}

	runEntCodegen(t, dir, modulePath)

	if out, err := runIn(dir, "go", "build", "./internal/..."); err != nil {
		t.Errorf("generated storage layer does not compile against the Ent client:\n%s", out)
	}
}

// TestAdapterDoesNotImportPackageMain guards a sharp edge found while building
// this coverage: the adapter imports the package each resource is declared in.
// A resource declared in package main yields `main "main"`, which is never a
// valid import. Real resources live in apis/<group>/<version>, so this only
// bites in scratch programs — but the failure mode is baffling if you hit it.
func TestAdapterDoesNotImportPackageMain(t *testing.T) {
	dir := generateProject(t, "adaptertest", true)

	content, err := os.ReadFile(filepath.Join(dir, "internal", "storage", "ent_adapter.go"))
	if err != nil {
		t.Fatalf("read adapter: %v", err)
	}
	if strings.Contains(string(content), `"main"`) {
		t.Errorf("adapter imports package main, which cannot compile:\n%s", content)
	}
}

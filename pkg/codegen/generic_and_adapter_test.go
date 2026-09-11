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
	if err := gen.RegisterResource(&testfixtures.BadWidget{}); err != nil {
		t.Fatalf("RegisterResource BadWidget: %v", err)
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

func generateVersionedProject(t *testing.T, modulePath string) string {
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
	gen.Config.VersioningEnabled = true

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if err := gen.RegisterResource(&testfixtures.VersionedWidget{}); err != nil {
		t.Fatalf("RegisterResource VersionedWidget: %v", err)
	}
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas: %v", err)
	}
	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("GenerateEntAdapter: %v", err)
	}
	if err := gen.GenerateEntHelpers(); err != nil {
		t.Fatalf("GenerateEntHelpers: %v", err)
	}
	if err := gen.GenerateStorage(); err != nil {
		t.Fatalf("GenerateStorage: %v", err)
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

const generatedStorageRuntimeTests = `package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"adaptertest/internal/storage/ent"
	entresource "adaptertest/internal/storage/ent/resource"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/openchami/fabrica/pkg/codegen/testfixtures"
	"github.com/openchami/fabrica/pkg/resource"
	_ "modernc.org/sqlite"
)

func newTestEntClient(t *testing.T) (*ent.Client, *sql.DB, context.Context) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		client.Close()
		t.Fatalf("migrate schema: %v", err)
	}
	SetEntClient(client)
	t.Cleanup(func() {
		SetEntClient(nil)
		client.Close()
	})

	return client, db, ctx
}

func widgetFixture(uid, name string, size int, phase string) *testfixtures.Widget {
	now := time.Unix(1700000000, 0).UTC()
	return &testfixtures.Widget{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "Widget",
			Metadata: resource.Metadata{
				Name:        name,
				UID:         uid,
				Labels:      map[string]string{"env": "test"},
				Annotations: map[string]string{"owner": "team-a"},
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		Spec:   testfixtures.WidgetSpec{Name: name, Size: size},
		Status: testfixtures.WidgetStatus{Phase: phase},
	}
}

func requireErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}

func requirePanicContains(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("expected panic containing %q", want)
		}
		if !strings.Contains(fmt.Sprint(got), want) {
			t.Fatalf("panic %q does not contain %q", got, want)
		}
	}()
	fn()
}

func TestGeneratedAdapterRoundTripAndErrors(t *testing.T) {
	client, _, ctx := newTestEntClient(t)

	widget := widgetFixture("uid-1", "widget-1", 3, "Ready")
	create, labels, annotations, err := ToEntResource(widget)
	if err != nil {
		t.Fatalf("ToEntResource: %v", err)
	}
	if labels["env"] != "test" || annotations["owner"] != "team-a" {
		t.Fatalf("metadata maps not returned: labels=%v annotations=%v", labels, annotations)
	}

	saved, err := create.Save(ctx)
	if err != nil {
		t.Fatalf("save ent resource: %v", err)
	}
	if saved.UID != "uid-1" || saved.Name != "widget-1" || saved.Kind != "Widget" {
		t.Fatalf("builder populated wrong fields: %#v", saved)
	}
	if len(saved.Status) == 0 {
		t.Fatal("expected status to be set")
	}
	if err := saveLabels(ctx, saved.ID, labels); err != nil {
		t.Fatalf("saveLabels: %v", err)
	}
	if err := saveAnnotations(ctx, saved.ID, annotations); err != nil {
		t.Fatalf("saveAnnotations: %v", err)
	}

	roundTripEnt, err := client.Resource.Query().Where(entresource.UIDEQ("uid-1")).WithLabels().WithAnnotations().Only(ctx)
	if err != nil {
		t.Fatalf("reload ent resource: %v", err)
	}
	roundTrip, err := FromEntResource(ctx, roundTripEnt)
	if err != nil {
		t.Fatalf("FromEntResource: %v", err)
	}
	got := roundTrip.(*testfixtures.Widget)
	if got.Spec.Name != "widget-1" || got.Spec.Size != 3 || got.Status.Phase != "Ready" {
		t.Fatalf("typed fields did not round-trip: %#v", got)
	}
	if got.Metadata.Labels["env"] != "test" || got.Metadata.Annotations["owner"] != "team-a" {
		t.Fatalf("edges did not round-trip: labels=%v annotations=%v", got.Metadata.Labels, got.Metadata.Annotations)
	}

	_, _, _, err = ToEntResource(struct{}{})
	requireErrorContains(t, err, "unsupported resource type")
	_, _, _, err = ToEntResource(&testfixtures.BadWidget{
		Resource: resource.Resource{APIVersion: "v1", Kind: "BadWidget", Metadata: resource.Metadata{Name: "bad", UID: "bad-1"}},
		Spec:     testfixtures.BadWidgetSpec{Broken: make(chan int)},
	})
	requireErrorContains(t, err, "failed to marshal spec")

	_, err = FromEntResource(ctx, &ent.Resource{Kind: "Widget", Spec: json.RawMessage(` + "`" + `{"name":` + "`" + `)})
	requireErrorContains(t, err, "failed to unmarshal spec for Widget")
	_, err = FromEntResource(ctx, &ent.Resource{Kind: "Bogus"})
	requireErrorContains(t, err, "unknown resource kind")
}

func TestGeneratedStorageCRUDAndBackendWrappers(t *testing.T) {
	_, db, ctx := newTestEntClient(t)

	created := widgetFixture("uid-1", "widget-1", 3, "Ready")
	if err := SaveWidget(ctx, created); err != nil {
		t.Fatalf("create widget: %v", err)
	}
	loaded, err := LoadWidget(ctx, "uid-1")
	if err != nil {
		t.Fatalf("load created widget: %v", err)
	}
	if loaded.Spec.Size != 3 || loaded.Status.Phase != "Ready" {
		t.Fatalf("created widget mismatch: %#v", loaded)
	}

	updated := widgetFixture("uid-1", "widget-renamed", 5, "Updated")
	updated.Metadata.Labels = map[string]string{"env": "prod"}
	updated.Metadata.Annotations = map[string]string{"owner": "team-b"}
	if err := SaveWidget(ctx, updated); err != nil {
		t.Fatalf("update widget: %v", err)
	}
	loaded, err = LoadWidget(ctx, "uid-1")
	if err != nil {
		t.Fatalf("load updated widget: %v", err)
	}
	if loaded.Metadata.Name != "widget-renamed" || loaded.Spec.Size != 5 || loaded.Status.Phase != "Updated" {
		t.Fatalf("update branch mismatch: %#v", loaded)
	}
	if loaded.Metadata.Labels["env"] != "prod" || loaded.Metadata.Annotations["owner"] != "team-b" {
		t.Fatalf("updated metadata mismatch: labels=%v annotations=%v", loaded.Metadata.Labels, loaded.Metadata.Annotations)
	}

	backendWidget := widgetFixture("uid-2", "widget-2", 8, "Ready")
	backendWidget.Metadata.Labels = nil
	backendWidget.Metadata.Annotations = nil
	backendData, err := json.Marshal(backendWidget)
	if err != nil {
		t.Fatalf("marshal backend widget: %v", err)
	}
	if err := Backend.Save(ctx, "Widget", "uid-2", backendData); err != nil {
		t.Fatalf("backend save: %v", err)
	}
	if _, err := Backend.Load(ctx, "Widget", "uid-2"); err != nil {
		t.Fatalf("backend load: %v", err)
	}
	if ok, err := Backend.Exists(ctx, "Widget", "uid-2"); err != nil || !ok {
		t.Fatalf("backend exists = %v, %v", ok, err)
	}
	if _, version, err := Backend.LoadWithVersion(ctx, "Widget", "uid-2", "requested"); err != nil || version != "requested" {
		t.Fatalf("LoadWithVersion version=%q err=%v", version, err)
	}
	if err := Backend.SaveWithVersion(ctx, "Widget", "uid-2", backendData, "ignored"); err != nil {
		t.Fatalf("SaveWithVersion: %v", err)
	}

	_, err = db.ExecContext(ctx,
		"INSERT INTO resources (uid, name, api_version, kind, resource_type, spec, created_at, updated_at, resource_version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"bad-json", "bad-json", "v1", "Widget", "Widget", ` + "`" + `{"name":"bad-json","size":"large"}` + "`" + `, time.Now(), time.Now(), "1")
	if err != nil {
		t.Fatalf("create malformed row: %v", err)
	}
	all, err := LoadAllWidgets(ctx)
	if err != nil {
		t.Fatalf("LoadAllWidgets: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected malformed row to be skipped, got %d resources", len(all))
	}

	q, err := QueryResourcesByLabels(ctx, "Widget", nil)
	if err != nil {
		t.Fatalf("QueryResourcesByLabels nil: %v", err)
	}
	if count, err := q.Count(ctx); err != nil || count != 3 {
		t.Fatalf("nil labels count=%d err=%v", count, err)
	}
	labelQuery, err := QueryResourcesByLabels(ctx, "Widget", map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("QueryResourcesByLabels prod: %v", err)
	}
	matched, err := labelQuery.WithLabels().WithAnnotations().All(ctx)
	if err != nil {
		t.Fatalf("execute label query: %v", err)
	}
	if len(matched) != 1 || matched[0].UID != "uid-1" {
		t.Fatalf("label query mismatch: %#v", matched)
	}

	if err := Backend.Delete(ctx, "Widget", "uid-2"); err != nil {
		t.Fatalf("backend delete: %v", err)
	}
	if err := DeleteWidget(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteWidget missing error=%v, want ErrNotFound", err)
	}
}

func TestGeneratedStorageNilClientDefaultsAndHelperErrors(t *testing.T) {
	SetEntClient(nil)
	requirePanicContains(t, "ent client not initialized", func() { _ = QueryResources(context.Background(), "Widget") })
	requirePanicContains(t, "ent client not initialized", func() { _ = NewStorageClient() })

	client, _, ctx := newTestEntClient(t)
	requireErrorContains(t, Backend.Save(ctx, "Bogus", "uid", json.RawMessage(` + "`" + `{}` + "`" + `)), "unsupported resource type")
	_, err := Backend.Load(ctx, "Bogus", "uid")
	requireErrorContains(t, err, "unsupported resource type")
	_, err = Backend.LoadAll(ctx, "Bogus")
	requireErrorContains(t, err, "unsupported resource type")
	requireErrorContains(t, Backend.Delete(ctx, "Bogus", "uid"), "unsupported resource type")
	requireErrorContains(t, NewStorageClient().Update(ctx, struct{}{}), "unsupported resource type")

	saved, err := client.Resource.Create().
		SetUID("uid-helpers").SetName("widget-helpers").SetAPIVersion("v1").SetKind("Widget").
		SetResourceType("Widget").SetSpec(json.RawMessage(` + "`" + `{"name":"widget-helpers","size":1}` + "`" + `)).Save(ctx)
	if err != nil {
		t.Fatalf("create helper resource: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close ent client: %v", err)
	}
	requireErrorContains(t, saveLabels(ctx, saved.ID, map[string]string{"env": "broken"}), "failed to delete old labels")
	requireErrorContains(t, saveAnnotations(ctx, saved.ID, map[string]string{"owner": "broken"}), "failed to delete old annotations")
}
`

const generatedVersionedStorageRuntimeTests = `package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/openchami/fabrica/pkg/codegen/testfixtures"
	"github.com/openchami/fabrica/pkg/resource"
	"versioningtest/internal/storage/ent"
	_ "modernc.org/sqlite"
)

func TestVersionedAdapterRoundTrip(t *testing.T) {
	db, err := sql.Open("sqlite", "file:versioned?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	defer client.Close()
	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	SetEntClient(client)
	defer SetEntClient(nil)

	now := time.Unix(1700000000, 0).UTC()
	resource := &testfixtures.VersionedWidget{
		APIVersion: "v1",
		Kind:       "VersionedWidget",
		Metadata: resource.Metadata{
			Name:        "versioned-widget",
			UID:         "versioned-1",
			Labels:      map[string]string{"env": "test"},
			Annotations: map[string]string{"owner": "team-a"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Spec:   testfixtures.WidgetSpec{Name: "versioned-widget", Size: 7},
		Status: testfixtures.WidgetStatus{Phase: "Ready"},
	}

	create, labels, annotations, err := ToEntResource(resource)
	if err != nil {
		t.Fatalf("ToEntResource: %v", err)
	}
	saved, err := create.Save(ctx)
	if err != nil {
		t.Fatalf("save ent resource: %v", err)
	}
	if err := saveLabels(ctx, saved.ID, labels); err != nil {
		t.Fatalf("saveLabels: %v", err)
	}
	if err := saveAnnotations(ctx, saved.ID, annotations); err != nil {
		t.Fatalf("saveAnnotations: %v", err)
	}
	loaded, err := LoadVersionedWidget(ctx, "versioned-1")
	if err != nil {
		t.Fatalf("LoadVersionedWidget: %v", err)
	}
	if loaded.APIVersion != "v1" || loaded.Kind != "VersionedWidget" || loaded.Metadata.UID != "versioned-1" {
		t.Fatalf("versioned metadata mismatch: %#v", loaded)
	}
	if loaded.Spec.Size != 7 || loaded.Status.Phase != "Ready" {
		t.Fatalf("versioned typed fields mismatch: %#v", loaded)
	}
	if loaded.Metadata.Labels["env"] != "test" || loaded.Metadata.Annotations["owner"] != "team-a" {
		t.Fatalf("versioned edges mismatch: labels=%v annotations=%v", loaded.Metadata.Labels, loaded.Metadata.Annotations)
	}
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

func TestGeneratedStorageRuntimeBehavior(t *testing.T) {
	const modulePath = "adaptertest"

	dir := generateProject(t, modulePath, true)
	prepareModule(t, dir, modulePath, true)
	runEntCodegen(t, dir, modulePath)
	writeFile(t, filepath.Join(dir, "internal", "storage", "storage_runtime_test.go"), generatedStorageRuntimeTests)
	if out, err := runIn(dir, "go", "get", "modernc.org/sqlite"); err != nil {
		t.Skipf("cannot resolve sqlite dependency for generated storage tests (%v): %s", err, out)
	}
	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed after writing generated storage tests (%v): %s", err, out)
	}

	if out, err := runIn(dir, "go", "test", "./internal/storage"); err != nil {
		t.Fatalf("generated storage runtime tests failed:\n%s", out)
	}
}

func TestGeneratedVersionedStorageRuntimeBehavior(t *testing.T) {
	const modulePath = "versioningtest"

	dir := generateVersionedProject(t, modulePath)
	prepareModule(t, dir, modulePath, true)
	runEntCodegen(t, dir, modulePath)
	writeFile(t, filepath.Join(dir, "internal", "storage", "versioned_storage_runtime_test.go"), generatedVersionedStorageRuntimeTests)
	if out, err := runIn(dir, "go", "get", "modernc.org/sqlite"); err != nil {
		t.Skipf("cannot resolve sqlite dependency for generated versioning tests (%v): %s", err, out)
	}
	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed after writing generated versioning tests (%v): %s", err, out)
	}

	if out, err := runIn(dir, "go", "test", "./internal/storage"); err != nil {
		t.Fatalf("generated versioning runtime tests failed:\n%s", out)
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

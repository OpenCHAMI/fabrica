// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRevisionEntGeneratedRuntimeParity(t *testing.T) {
	dir := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	writeRevisionEntFixture(t, dir, root)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	gen := NewGenerator(dir, "main", "example.com/entrevisionfixture")
	gen.SetStorageType("ent")
	gen.DBDriver = "sqlite3"
	gen.Config.VersioningEnabled = true
	gen.Resources = []ResourceMetadata{{
		Name: "Node", PluralName: "nodes", Package: "example.com/entrevisionfixture/apis/v1", PackageAlias: "v1",
		TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", URLPath: "/nodes",
		StorageName: "Node", RevisioningEnabled: true, BareNameSelector: "default",
	}}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas() error = %v", err)
	}
	storageDir := filepath.Join(dir, "internal", "storage")
	if err := gen.executeTemplate("storageEntRevisions", filepath.Join(storageDir, "ent_revisions_generated.go"), gen.globalTemplateData("storage/ent_revisions.go.tmpl")); err != nil {
		t.Fatalf("generate Ent revision storage: %v", err)
	}

	for _, args := range [][]string{
		{"run", "-mod=mod", "entgo.io/ent/cmd/ent", "generate", "./internal/storage/ent/schema"},
		{"mod", "tidy"},
		{"test", "-count=1", "./internal/storage"},
	} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.6")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("execute generated Ent revision go %v: %v\n%s", args, err, output)
		}
	}
}

func writeRevisionEntFixture(t *testing.T, dir, root string) {
	t.Helper()
	files := map[string]string{
		"go.mod": fmt.Sprintf(`module example.com/entrevisionfixture

go 1.26.6

require (
	entgo.io/ent v0.14.5
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/openchami/fabrica v0.0.0
)

replace github.com/openchami/fabrica => %s
`, root),
		"apis/v1/types.go": `package v1
import "github.com/openchami/fabrica/pkg/resource"
type NodeSpec struct { Value string ` + "`json:\"value\"`" + ` }
type NodeStatus struct { Phase string ` + "`json:\"phase,omitempty\"`" + ` }
type Node struct { APIVersion string ` + "`json:\"apiVersion\"`" + `; Kind string ` + "`json:\"kind\"`" + `; Metadata resource.Metadata ` + "`json:\"metadata\"`" + `; Spec NodeSpec ` + "`json:\"spec\"`" + `; Status NodeStatus ` + "`json:\"status\"`" + ` }
`,
		"internal/storage/support.go": `package storage
import (
	"fmt"
	"example.com/entrevisionfixture/internal/storage/ent"
)
var entClient *ent.Client
func ensureBackendReady() error { if entClient == nil { return fmt.Errorf("ent client not initialized") }; return nil }
`,
		"internal/storage/revision_runtime_test.go": `package storage
import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"example.com/entrevisionfixture/internal/storage/ent"
	v1 "example.com/entrevisionfixture/apis/v1"
	dialectsql "entgo.io/ent/dialect/sql"
	"github.com/openchami/fabrica/pkg/revision"
	_ "github.com/mattn/go-sqlite3"
)
func node(value string) *v1.Node { return &v1.Node{APIVersion:"v1", Kind:"Node", Spec:v1.NodeSpec{Value:value}} }
func TestEntRevisionParity(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "revision.db")+"?_fk=1&_busy_timeout=5000&_journal_mode=WAL")
	if err != nil { t.Fatal(err) }
	db.SetMaxOpenConns(4)
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB("sqlite3", db)))
	t.Cleanup(func(){ _ = client.Close() })
	if err := client.Schema.Create(ctx); err != nil { t.Fatal(err) }
	entClient = client
	first, createETag, err := CreateNodeSeries(ctx, "production", "one", node("one"))
	if err != nil { t.Fatal(err) }
	if first.Metadata.Number != 1 { t.Fatalf("first number = %d", first.Metadata.Number) }
	if _, storedETag, err := GetNodeSeries(ctx, "production"); err != nil || createETag != storedETag { t.Fatalf("create ETag = %q, stored = %q, err=%v", createETag, storedETag, err) }

	type result struct { record revision.Record[*v1.Node]; created bool; etag string; err error }
	results := make(chan result, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 2 { wg.Add(1); go func(){ defer wg.Done(); <-start; r, c, e, err := EnsureNodeRevisionByName(ctx, "production", "two", node("two")); results <- result{r,c,e,err} }() }
	close(start); wg.Wait(); close(results)
	var second revision.Record[*v1.Node]
	createdCount := 0
	var ensureETags []string
	for got := range results { if got.err != nil { t.Fatal(got.err) }; if got.created { createdCount++ }; if second.Metadata.UID == "" { second = got.record } else if second.Metadata.UID != got.record.Metadata.UID { t.Fatalf("same name produced UIDs %s and %s", second.Metadata.UID, got.record.Metadata.UID) }; ensureETags = append(ensureETags, got.etag) }
	if createdCount != 1 || second.Metadata.Number != 2 { t.Fatalf("same-name ensure created=%d number=%d", createdCount, second.Metadata.Number) }
	_, storedETag, err := GetNodeSeries(ctx, "production")
	if err != nil { t.Fatal(err) }
	for _, returnedETag := range ensureETags { if returnedETag != storedETag { t.Fatalf("ensure ETag = %q, stored = %q", returnedETag, storedETag) } }
	if _, _, _, err := EnsureNodeRevisionByName(ctx, "production", "two", node("changed")); !errors.Is(err, revision.ErrRevisionNameConflict) { t.Fatalf("conflict error = %v", err) }
	third, created, etag, err := EnsureNodeRevisionByName(ctx, "production", "three", node("three"))
	if err != nil || !created || third.Metadata.Number != 3 { t.Fatalf("third = %#v created=%t err=%v", third.Metadata, created, err) }
	if _, storedETag, err = GetNodeSeries(ctx, "production"); err != nil || etag != storedETag { t.Fatalf("ensure ETag = %q, stored = %q, err=%v", etag, storedETag, err) }
	if _, err := RetireNodeRevision(ctx, "production", first.Metadata.UID); !errors.Is(err, revision.ErrRevisionInUse) { t.Fatalf("retire default error = %v", err) }
	if _, err := RetireNodeRevision(ctx, "production", third.Metadata.UID); !errors.Is(err, revision.ErrRevisionInUse) { t.Fatalf("retire latest error = %v", err) }
	latest, err := ResolveNodeRevision(ctx, revision.Reference{Name:"production", Selector:revision.SelectorLatest})
	if err != nil || latest.Metadata.UID != third.Metadata.UID { t.Fatalf("latest = %#v err=%v", latest.Metadata, err) }
	_, promotionETag, err := GetNodeSeries(ctx, "production")
	if err != nil { t.Fatal(err) }
	promotions := make(chan error, 2)
	var promoteWG sync.WaitGroup
	promoteStart := make(chan struct{})
	for _, uid := range []string{first.Metadata.UID, second.Metadata.UID} { promoteWG.Add(1); go func(){ defer promoteWG.Done(); <-promoteStart; _, returnedETag, err := PromoteNodeDefault(ctx, "production", uid, promotionETag); if err == nil { _, storedETag, getErr := GetNodeSeries(ctx, "production"); if getErr != nil { err = getErr } else if returnedETag != storedETag { err = errors.New("promotion ETag does not match committed series") } }; promotions <- err }() }
	close(promoteStart)
	promoteWG.Wait(); close(promotions)
	succeeded, preconditioned := 0, 0
	var promotionErrors []error
	for err := range promotions { promotionErrors = append(promotionErrors, err); if err == nil { succeeded++ } else if errors.Is(err, revision.ErrPreconditionFailed) { preconditioned++ } else { t.Fatalf("concurrent promotion error = %v", err) } }
	if succeeded != 1 || preconditioned != 1 { t.Fatalf("concurrent promotions succeeded=%d preconditioned=%d errors=%v", succeeded, preconditioned, promotionErrors) }
	current, currentETag, err := GetNodeSeries(ctx, "production")
	if err != nil { t.Fatal(err) }
	if current.DefaultRevisionUID != second.Metadata.UID { if _, currentETag, err = PromoteNodeDefault(ctx, "production", second.Metadata.UID, currentETag); err != nil { t.Fatal(err) } }
	if _, _, err := PromoteNodeDefault(ctx, "production", first.Metadata.UID, etag); !errors.Is(err, revision.ErrPreconditionFailed) { t.Fatalf("stale promotion error = %v", err) }
	retired, err := RetireNodeRevision(ctx, "production", first.Metadata.UID)
	if err != nil || retired.Metadata.RetiredAt == nil { t.Fatalf("retired = %#v err=%v", retired.Metadata, err) }
	if _, err := GetNodeRevision(ctx, "production", first.Metadata.UID); !errors.Is(err, revision.ErrRevisionRetired) { t.Fatalf("retired get error = %v", err) }
	if _, err := ResolveNodeRevision(ctx, revision.Reference{Name:"production", UID:first.Metadata.UID}); !errors.Is(err, revision.ErrRevisionRetired) { t.Fatalf("retired UID resolve error = %v", err) }
	if _, err := ResolveNodeRevision(ctx, revision.Reference{Name:"production", RevisionName:"one"}); !errors.Is(err, revision.ErrRevisionRetired) { t.Fatalf("retired name resolve error = %v", err) }
	if _, _, _, err := EnsureNodeRevisionByName(ctx, "production", "one", node("replacement")); !errors.Is(err, revision.ErrRevisionNameConflict) { t.Fatalf("retired rebind error = %v", err) }
	fourth, created, _, err := EnsureNodeRevisionByName(ctx, "production", "four", node("four"))
	if err != nil || !created || fourth.Metadata.Number != 4 { t.Fatalf("fourth = %#v created=%t err=%v", fourth.Metadata, created, err) }
	records, err := ListNodeRevisions(ctx, "production")
	if err != nil || len(records) != 4 || records[0].Metadata.RetiredAt == nil { t.Fatalf("records = %#v err=%v", records, err) }
	retirements := make(chan error, 2)
	var retireWG sync.WaitGroup
	retireStart := make(chan struct{})
	for range 2 { retireWG.Add(1); go func(){ defer retireWG.Done(); <-retireStart; _, err := RetireNodeRevision(ctx, "production", third.Metadata.UID); retirements <- err }() }
	close(retireStart); retireWG.Wait(); close(retirements)
	retiredCount, alreadyRetiredCount := 0, 0
	for err := range retirements { if err == nil { retiredCount++ } else if errors.Is(err, revision.ErrRevisionRetired) { alreadyRetiredCount++ } else { t.Fatalf("concurrent retirement error = %v", err) } }
	if retiredCount != 1 || alreadyRetiredCount != 1 { t.Fatalf("concurrent retirement retired=%d already-retired=%d", retiredCount, alreadyRetiredCount) }

	if _, _, err := CreateNodeSeries(ctx, "conflict", "base", node("base")); err != nil { t.Fatal(err) }
	conflicts := make(chan result, 2)
	var conflictWG sync.WaitGroup
	conflictStart := make(chan struct{})
	for _, value := range []string{"left", "right"} { conflictWG.Add(1); go func(){ defer conflictWG.Done(); <-conflictStart; r, c, e, err := EnsureNodeRevisionByName(ctx, "conflict", "release", node(value)); conflicts <- result{r,c,e,err} }() }
	close(conflictStart); conflictWG.Wait(); close(conflicts)
	conflictCreated, conflictRejected := 0, 0
	for got := range conflicts { if got.err == nil && got.created { conflictCreated++ } else if errors.Is(got.err, revision.ErrRevisionNameConflict) { conflictRejected++ } else { t.Fatalf("different-content race = created %t err %v", got.created, got.err) } }
	if conflictCreated != 1 || conflictRejected != 1 { t.Fatalf("different-content race created=%d conflicted=%d", conflictCreated, conflictRejected) }
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

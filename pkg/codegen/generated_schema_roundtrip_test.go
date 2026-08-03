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
	"time"

	"github.com/openchami/fabrica/pkg/annotations"
)

// This test answers the last question the others cannot: does a schema fabrica
// generates actually WORK?
//
// Compiling proves the file builds. It does not prove Ent accepts the schema,
// that the columns and indexes can be created, or that a value written through
// the generated client comes back as the same Go value. Those are separate
// failure modes — a column typed as text will compile and migrate happily while
// silently mangling every timestamp.
//
// The pipeline here is the real one:
//
//	fabrica generates schema  ->  entc generates a client  ->  migrate SQLite
//	  ->  create a row  ->  read it back  ->  compare every field
//
// It is slow (Ent codegen plus two module builds) and needs entgo.io/ent,
// ariga.io/atlas and a SQLite driver resolvable. Skipped in -short mode, and
// skipped rather than failed when dependencies cannot be fetched.

// RoundTripSpec exercises one field of every Go type the dedicated schema
// template maps. The type name is exported deliberately: Ent's loader ignores
// unexported schema types, so an unexported fixture silently produces
// "no schema found".
type RoundTripSpec struct {
	Subject        string            `json:"subject" validate:"required"`
	UsageCount     int               `json:"usage_count"`
	Revoked        bool              `json:"revoked"`
	SequenceNumber int64             `json:"sequence_number"`
	Weight         float64           `json:"weight"`
	TTL            time.Duration     `json:"ttl"`
	IssuedAt       time.Time         `json:"issued_at" validate:"required"`
	ConsumedAt     *time.Time        `json:"consumed_at"`
	Scopes         []string          `json:"scopes"`
	Fingerprint    []byte            `json:"fingerprint"`
	ReplayAttempts []time.Time       `json:"replay_attempts"`
	Labels         map[string]string `json:"labels"`
}

// RoundTrip is the resource under test.
type RoundTrip struct {
	Spec RoundTripSpec
}

// crudProgram writes a row through the generated client, reads it back, and
// reports one line per field. Any mismatch exits non-zero.
const crudProgram = `package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"roundtrip/ent"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "file:ent?mode=memory&cache=shared&_fk=1")
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

	ttl := 90 * time.Minute
	issued := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

	created, err := client.RoundTrip.Create().
		SetUID("uid-1").
		SetName("row-1").
		SetSubject("boot-service").
		SetTTL(int64(ttl)).
		SetIssuedAt(issued).
		SetScopes([]string{"read", "write"}).
		SetFingerprint([]byte{0xde, 0xad}).
		SetReplayAttempts([]time.Time{issued}).
		SetLabels(map[string]string{"env": "test"}).
		SetSequenceNumber(42).
		SetWeight(1.5).
		SetUsageCount(3).
		SetRevoked(false).
		Save(ctx)
	if err != nil {
		log.Fatalf("CREATE FAILED: %v", err)
	}

	got, err := client.RoundTrip.Get(ctx, created.ID)
	if err != nil {
		log.Fatalf("READ FAILED: %v", err)
	}

	fail := 0
	check := func(name string, ok bool, detail string) {
		status := "ok"
		if !ok {
			status = "FAIL"
			fail++
		}
		fmt.Printf("%s %s %s\n", status, name, detail)
	}

	check("time.Duration", time.Duration(got.TTL) == ttl, time.Duration(got.TTL).String())
	check("time.Time", got.IssuedAt.UTC().Equal(issued), got.IssuedAt.UTC().String())
	check("ptr-nil", got.ConsumedAt == nil, "nil")
	check("[]string", len(got.Scopes) == 2 && got.Scopes[0] == "read", fmt.Sprint(got.Scopes))
	check("[]byte", len(got.Fingerprint) == 2 && got.Fingerprint[0] == 0xde, fmt.Sprintf("%x", got.Fingerprint))
	check("[]time.Time", len(got.ReplayAttempts) == 1, fmt.Sprint(len(got.ReplayAttempts)))
	check("map", got.Labels["env"] == "test", fmt.Sprint(got.Labels))
	check("int64", got.SequenceNumber == 42, fmt.Sprint(got.SequenceNumber))
	check("float64", got.Weight == 1.5, fmt.Sprint(got.Weight))
	check("int", got.UsageCount == 3, fmt.Sprint(got.UsageCount))
	check("bool", !got.Revoked, fmt.Sprint(got.Revoked))

	consumed := issued.Add(time.Minute)
	upd, err := got.Update().SetConsumedAt(consumed).Save(ctx)
	if err != nil {
		log.Fatalf("UPDATE FAILED: %v", err)
	}
	check("ptr-set", upd.ConsumedAt != nil && upd.ConsumedAt.UTC().Equal(consumed), fmt.Sprint(upd.ConsumedAt))

	if fail > 0 {
		log.Fatalf("%d field(s) did not round-trip", fail)
	}
	fmt.Println("all round-trips ok")
}
`

const entgenProgram = `package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	if err := entc.Generate("../../schema", &gen.Config{Target: "../../ent", Package: "roundtrip/ent"}); err != nil {
		log.Fatalf("ent codegen: %v", err)
	}
}
`

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// runIn executes a command in dir, returning combined output.
func runIn(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	return string(out), err
}

// TestGeneratedSchemaRoundTripsThroughEnt is the end-to-end proof.
func TestGeneratedSchemaRoundTripsThroughEnt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end Ent round-trip in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain available")
	}

	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	subject := annotations.NewFieldAnnotations("Subject")
	subject.Size = 253
	subject.NotNull = true
	subject.Index = &annotations.IndexConfig{Type: annotations.IndexTypeBTree}
	annots.Fields["Subject"] = subject

	issued := annotations.NewFieldAnnotations("IssuedAt")
	issued.Immutable = true
	annots.Fields["IssuedAt"] = issued

	annots.Indexes = []*annotations.CompositeIndex{
		{Fields: []string{"Subject", "SequenceNumber"}, Name: "idx_subject_seq", Type: annotations.IndexTypeBTree},
	}

	if err := annotations.Validate(annots); err != nil {
		t.Fatalf("fixture failed validation: %v", err)
	}

	schema := generateDedicatedSchema(t, &RoundTrip{}, "RoundTrip", annots)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "schema", "roundtrip.go"), string(schema))
	writeFile(t, filepath.Join(dir, "cmd", "entgen", "main.go"), entgenProgram)
	writeFile(t, filepath.Join(dir, "cmd", "crud", "main.go"), crudProgram)
	writeFile(t, filepath.Join(dir, "go.mod"), "module roundtrip\n\ngo 1.24\n")

	// Resolve dependencies; skip if the environment cannot.
	for _, args := range [][]string{
		{"get", "entgo.io/ent@" + entVersion},
		{"get", "ariga.io/atlas"},
		{"get", "golang.org/x/tools"},
		{"get", "golang.org/x/crypto"},
		{"get", "modernc.org/sqlite"},
	} {
		if out, err := runIn(dir, "go", args...); err != nil {
			t.Skipf("cannot resolve dependencies for the round-trip test (%v): %s", err, out)
		}
	}
	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed (%v): %s", err, out)
	}

	// Ent's own codegen must accept the schema. This catches semantic problems
	// a compiler cannot see: unknown index fields, duplicate columns, field
	// options Ent rejects.
	if out, err := runIn(filepath.Join(dir, "cmd", "entgen"), "go", "run", "."); err != nil {
		t.Fatalf("Ent rejected the generated schema:\n%s", out)
	}

	// Migrate and round-trip every field through a real database.
	out, err := runIn(filepath.Join(dir, "cmd", "crud"), "go", "run", ".")
	if err != nil {
		t.Fatalf("round-trip failed:\n%s", out)
	}

	t.Logf("round-trip results:\n%s", out)

	if !strings.Contains(out, "migrate ok") {
		t.Error("migration did not report success")
	}
	if !strings.Contains(out, "all round-trips ok") {
		t.Errorf("not every field round-tripped:\n%s", out)
	}
	if strings.Contains(out, "FAIL ") {
		t.Errorf("a field did not round-trip:\n%s", out)
	}
}

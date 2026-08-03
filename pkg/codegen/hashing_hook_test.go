// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

// The hashing hook is the last piece of the dedicated path with no runtime
// proof, and the reason it stayed broken so long is that nothing ever ran it.
//
// Compiling proves the hook exists. It does not prove the hook FIRES. A hook
// whose mutation lookup misses does nothing at all — no error, no warning — and
// the value a resource author annotated as hashed is written to the database in
// plaintext. For a token or password column that is a silent security failure,
// which is exactly the kind of bug a compile-only test cannot see.

// HashSpec covers both algorithms and, deliberately, a field whose JSON name
// differs from its Go name.
//
// That last one is the regression guard for a real bug: the hook used to look
// the field up by Go name, while Ent names its accessors after the COLUMN. For
// `Secret string ` + "`json:\"secret_value\"`" + ` the lookup silently missed
// and the secret was stored unhashed.
type HashSpec struct {
	Password string `json:"password" validate:"required"`
	APIKey   string `json:"api_key" validate:"required"`
	Secret   string `json:"secret_value" validate:"required"`
	Plain    string `json:"plain"`
}

// HashRes is the resource under test.
type HashRes struct {
	Spec HashSpec
}

// hashCRUD writes plaintext through the generated client and asserts what
// actually landed in the database.
const hashCRUD = `package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"hashtest/ent"
	// Ent registers schema hooks and field defaults from the generated runtime
	// package. Without this blank import the hashing hook never runs and
	// defaults are uninitialized.
	_ "hashtest/ent/runtime"
	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	db, err := sql.Open("sqlite", "file:h?mode=memory&cache=shared&_fk=1")
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

	const (
		passwordPlain = "correct-horse-battery-staple"
		apiKeyPlain   = "ak_live_0123456789"
		secretPlain   = "s3cr3t-value"
		plainPlain    = "not-secret"
	)

	created, err := client.HashRes.Create().
		SetUID("uid-1").SetName("row-1").
		SetPassword(passwordPlain).
		SetAPIKey(apiKeyPlain).
		SetSecretValue(secretPlain).
		SetPlain(plainPlain).
		Save(ctx)
	if err != nil {
		log.Fatalf("CREATE FAILED: %v", err)
	}

	got, err := client.HashRes.Get(ctx, created.ID)
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

	// bcrypt: stored value must verify against the plaintext, and must not BE
	// the plaintext.
	check("bcrypt not plaintext", got.Password != passwordPlain, trunc(got.Password))
	check("bcrypt verifies",
		bcrypt.CompareHashAndPassword([]byte(got.Password), []byte(passwordPlain)) == nil,
		trunc(got.Password))

	// sha256: stored value must equal hex(sha256(plaintext)).
	apiSum := sha256.Sum256([]byte(apiKeyPlain))
	check("sha256 api_key", got.APIKey == hex.EncodeToString(apiSum[:]), trunc(got.APIKey))

	// The regression case: Go name Secret, column secret_value.
	secSum := sha256.Sum256([]byte(secretPlain))
	check("sha256 name-mismatch", got.SecretValue == hex.EncodeToString(secSum[:]), trunc(got.SecretValue))
	check("mismatch not plaintext", got.SecretValue != secretPlain, trunc(got.SecretValue))

	// An unannotated field must be left exactly as written.
	check("unannotated untouched", got.Plain == plainPlain, got.Plain)

	// The hook fires on create only. Updating must not re-hash an already
	// hashed value, which would make the stored digest unverifiable.
	stored := got.Password
	upd, err := got.Update().SetPlain("changed").Save(ctx)
	if err != nil {
		log.Fatalf("UPDATE FAILED: %v", err)
	}
	check("update does not rehash", upd.Password == stored, trunc(upd.Password))
	check("update still verifies",
		bcrypt.CompareHashAndPassword([]byte(upd.Password), []byte(passwordPlain)) == nil,
		trunc(upd.Password))

	if fail > 0 {
		log.Fatalf("%d hashing check(s) failed", fail)
	}
	fmt.Println("all hashing checks ok")
}

func trunc(s string) string {
	if len(s) > 24 {
		return s[:24] + "..."
	}
	return s
}
`

// TestHashingHookActuallyHashes runs the full pipeline and inspects what was
// written to the database.
func TestHashingHookActuallyHashes(t *testing.T) {
	const modulePath = "hashtest"

	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	password := annotations.NewFieldAnnotations("Password")
	password.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		// Cost 4 is bcrypt's minimum — this test hashes on every run and the
		// cost factor is not what is under test.
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmBcrypt, Cost: 4},
	}
	password.Sensitive = true
	annots.Fields["Password"] = password

	apiKey := annotations.NewFieldAnnotations("APIKey")
	apiKey.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmSHA256},
	}
	annots.Fields["APIKey"] = apiKey

	secret := annotations.NewFieldAnnotations("Secret")
	secret.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmSHA256},
	}
	annots.Fields["Secret"] = secret

	if err := annotations.Validate(annots); err != nil {
		t.Fatalf("fixture failed validation: %v", err)
	}

	schema := generateDedicatedSchema(t, &HashRes{}, "HashRes", annots)

	// The hook must key off the column name, not the Go field name.
	if !strings.Contains(string(schema), `m.Field("secret_value")`) {
		t.Errorf("hook does not look up the mismatched field by column name:\n%s", schema)
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "schema", "hashres.go"), string(schema))
	writeFile(t, filepath.Join(dir, "cmd", "crud", "main.go"), hashCRUD)
	writeFile(t, filepath.Join(dir, "go.mod"), "module "+modulePath+"\n\ngo 1.24\n")

	for _, args := range [][]string{
		{"get", "entgo.io/ent@" + entVersion},
		{"get", "ariga.io/atlas"},
		{"get", "golang.org/x/tools"},
		{"get", "golang.org/x/crypto"},
		{"get", "modernc.org/sqlite"},
	} {
		if out, err := runIn(dir, "go", args...); err != nil {
			t.Skipf("cannot resolve dependencies (%v): %s", err, out)
		}
	}

	entgen := `package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	if err := entc.Generate("../../schema", &gen.Config{Target: "../../ent", Package: "` + modulePath + `/ent"}); err != nil {
		log.Fatal(err)
	}
}
`
	writeFile(t, filepath.Join(dir, "cmd", "entgen", "main.go"), entgen)

	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed (%v): %s", err, out)
	}
	if out, err := runIn(filepath.Join(dir, "cmd", "entgen"), "go", "run", "."); err != nil {
		t.Fatalf("Ent rejected the hashing schema:\n%s", out)
	}
	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed (%v): %s", err, out)
	}

	out, err := runIn(filepath.Join(dir, "cmd", "crud"), "go", "run", ".")
	if err != nil {
		t.Fatalf("hashing hook did not behave correctly:\n%s", out)
	}

	t.Logf("hashing results:\n%s", out)

	if !strings.Contains(out, "all hashing checks ok") || strings.Contains(out, "FAIL ") {
		t.Errorf("hashing checks incomplete:\n%s", out)
	}
}

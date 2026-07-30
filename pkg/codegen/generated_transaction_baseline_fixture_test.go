// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

const generatedTransactionBaselineTest = `package storage

import (
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"

	"example.com/generated-annotation-acceptance/internal/storage/ent"
	"example.com/generated-annotation-acceptance/internal/storage/ent/resource"
)

func TestGeneratedTransactionHelper_commits_on_nil_and_rolls_back_on_error(t *testing.T) {
	// Given
	db, err := dialectsql.Open(dialect.SQLite, "file:transaction-baseline?mode=memory&cache=shared&_fk=1")
	if err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(db))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	SetEntClient(client)

	// When
	err = WithTx(t.Context(), func(tx *ent.Tx) error {
		_, saveErr := tx.Resource.Create().SetUID("committed").SetName("committed").SetKind("Legacy").SetResourceType("Legacy").SetSpec([]byte(` + "`{}`" + `)).Save(t.Context())
		return saveErr
	})
	if err != nil { t.Fatal(err) }
	wantRollback := errors.New("rollback baseline")
	err = WithTx(t.Context(), func(tx *ent.Tx) error {
		if _, saveErr := tx.Resource.Create().SetUID("rolled-back").SetName("rolled-back").SetKind("Legacy").SetResourceType("Legacy").SetSpec([]byte(` + "`{}`" + `)).Save(t.Context()); saveErr != nil { return saveErr }
		return wantRollback
	})

	// Then
	if !errors.Is(err, wantRollback) { t.Fatalf("rollback error = %v", err) }
	committed, err := client.Resource.Query().Where(resource.UIDEQ("committed")).Exist(t.Context())
	if err != nil { t.Fatal(err) }
	rolledBack, err := client.Resource.Query().Where(resource.UIDEQ("rolled-back")).Exist(t.Context())
	if err != nil { t.Fatal(err) }
	if !committed || rolledBack { t.Fatalf("committed=%t rolledBack=%t", committed, rolledBack) }
	t.Log("transaction commit and rollback baseline")
}
`

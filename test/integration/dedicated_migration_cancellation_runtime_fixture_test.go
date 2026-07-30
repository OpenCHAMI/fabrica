// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const dedicatedMigrationCancellationRuntimeTests = `
func cancelAfterTokenInserts(cancel context.CancelFunc, trigger int) ent.Hook {
	inserted := 0
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			value, err := next.Mutate(ctx, mutation)
			if err == nil {
				inserted++
				if inserted == trigger { cancel() }
			}
			return value, err
		})
	}
}

func TestDedicatedMigration_mid_batch_cancellation_rolls_back_insert(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	seedGenericToken(t, client, genericToken("cancel-first", "first-password"))
	seedGenericToken(t, client, genericToken("cancel-second", "second-password"))
	sourceSnapshot := snapshotGenericSources(t, db)
	ctx, cancel := context.WithCancel(t.Context())
	client.Token.Use(cancelAfterTokenInserts(cancel, 1))

	// When
	report, err := MigrateTokenFromGeneric(ctx, DedicatedMigrationOptions{Limit: 10})
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	if !errors.Is(err, context.Canceled) { t.Fatalf("cancel error=%v report=%#v", err, report) }
	if report.Copied != 0 || report.NextAfterID != 0 || generic != 2 || dedicated != 0 { t.Fatalf("report=%#v raw=%d/%d", report, generic, dedicated) }
	requireGenericSourcesUnchanged(t, "mid-batch cancellation", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("mid batch cancellation rollback source preserved")
}

func TestDedicatedMigration_final_save_cancellation_rolls_back_before_commit(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	seedGenericToken(t, client, genericToken("cancel-final", "final-password"))
	sourceSnapshot := snapshotGenericSources(t, db)
	ctx, cancel := context.WithCancel(t.Context())
	client.Token.Use(cancelAfterTokenInserts(cancel, 1))

	// When
	report, err := MigrateTokenFromGeneric(ctx, DedicatedMigrationOptions{Limit: 10})
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	if !errors.Is(err, context.Canceled) { t.Fatalf("cancel error=%v report=%#v", err, report) }
	if report.Copied != 0 || report.NextAfterID != 0 || generic != 1 || dedicated != 0 { t.Fatalf("report=%#v raw=%d/%d", report, generic, dedicated) }
	if len(report.Failures) != 1 || report.Failures[0].SourceUID != "cancel-final" || report.Failures[0].Stage != "pre-commit-context" { t.Fatalf("report=%#v", report) }
	requireGenericSourcesUnchanged(t, "final-save cancellation", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("final save cancellation rollback source preserved")
}

func TestDedicatedMigration_retry_cursor_copies_all_rows_exactly_once(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	for _, uid := range []string{"page-1a", "page-1b", "page-2a", "page-2b", "page-3a"} {
		seedGenericToken(t, client, genericToken(uid, "password-"+uid))
	}
	if _, err := db.ExecContext(t.Context(), "UPDATE resources SET spec = ? WHERE uid = ?", []byte(` + "`{\"displayName\":`" + `), "page-2b"); err != nil { t.Fatal(err) }

	// When
	page1, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 2})
	if err != nil { t.Fatal(err) }
	page2Failure, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 2, AfterID: page1.NextAfterID})
	if err == nil { t.Fatal("expected page 2 conversion failure") }
	fixedSpec, marshalErr := json.Marshal(genericToken("page-2b", "password-page-2b").Spec)
	if marshalErr != nil { t.Fatal(marshalErr) }
	if _, fixErr := db.ExecContext(t.Context(), "UPDATE resources SET spec = ? WHERE uid = ?", fixedSpec, "page-2b"); fixErr != nil { t.Fatal(fixErr) }
	page2, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 2, AfterID: page2Failure.NextAfterID})
	if err != nil { t.Fatal(err) }
	page3, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 2, AfterID: page2.NextAfterID})
	if err != nil { t.Fatal(err) }

	// Then
	if page1.Copied != 2 || page1.NextAfterID == 0 { t.Fatalf("page1=%#v", page1) }
	if page2Failure.Copied != 0 || page2Failure.NextAfterID != page1.NextAfterID { t.Fatalf("page2 failure=%#v page1=%#v", page2Failure, page1) }
	if page2.Copied != 2 || page2.NextAfterID <= page1.NextAfterID { t.Fatalf("page2=%#v", page2) }
	if page3.Copied != 1 || page3.NextAfterID <= page2.NextAfterID { t.Fatalf("page3=%#v", page3) }
	wantUIDs := []string{"page-1a", "page-1b", "page-2a", "page-2b", "page-3a"}
	if got := rawDedicatedUIDs(t, db); !reflect.DeepEqual(got, wantUIDs) { t.Fatalf("dedicated UIDs=%v want=%v", got, wantUIDs) }
	t.Log("multi page retry cursor copies exactly once")
}

func TestDedicatedMigration_wrong_type_returns_typed_source_failure(t *testing.T) {
	// When
	_, err := asTokenMigrationResource("wrong-type-source", struct{}{})

	// Then
	var typeErr *TokenMigrationTypeError
	if !errors.As(err, &typeErr) { t.Fatalf("wrong type error=%T %v", err, err) }
	if typeErr.SourceUID != "wrong-type-source" || typeErr.Expected != "*v1.Token" { t.Fatalf("typed error=%#v", typeErr) }
	t.Log("wrong type returns typed source failure")
}
`

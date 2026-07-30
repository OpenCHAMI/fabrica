// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const dedicatedMigrationRuntimeTests = `func TestDedicatedMigration_preview_copy_rerun_and_authoritative_load(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	first := genericToken("token-1", "plaintext-one")
	second := genericToken("token-2", "plaintext-two")
	seedGenericToken(t, client, first)
	seedGenericToken(t, client, second)
	sourceSnapshot := snapshotGenericSources(t, db)
	beforeGeneric, beforeDedicated := rawMigrationCounts(t, db)

	// When
	preview, err := PreviewTokenMigration(t.Context(), DedicatedMigrationOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	previewGeneric, previewDedicated := rawMigrationCounts(t, db)
	requireGenericSourcesUnchanged(t, "preview", sourceSnapshot, snapshotGenericSources(t, db))
	migrated, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	afterGeneric, afterDedicated := rawMigrationCounts(t, db)
	requireGenericSourcesUnchanged(t, "success", sourceSnapshot, snapshotGenericSources(t, db))
	rerun, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	rerunGeneric, rerunDedicated := rawMigrationCounts(t, db)
	requireGenericSourcesUnchanged(t, "rerun", sourceSnapshot, snapshotGenericSources(t, db))

	// Then
	requireReport(t, preview, 2, 2, 0, 0)
	requireReport(t, migrated, 2, 2, 2, 0)
	requireReport(t, rerun, 2, 0, 0, 2)
	if preview.NextAfterID == 0 || migrated.NextAfterID != preview.NextAfterID || rerun.NextAfterID != preview.NextAfterID { t.Fatalf("commit-aware success cursors preview=%d migrated=%d rerun=%d", preview.NextAfterID, migrated.NextAfterID, rerun.NextAfterID) }
	if strings.Join(preview.SourceUIDs, ",") != "token-1,token-2" || strings.Join(migrated.SourceUIDs, ",") != "token-1,token-2" { t.Fatalf("preview=%#v migrated=%#v", preview.SourceUIDs, migrated.SourceUIDs) }
	if beforeGeneric != 2 || beforeDedicated != 0 || previewGeneric != 2 || previewDedicated != 0 || afterGeneric != 2 || afterDedicated != 2 || rerunGeneric != 2 || rerunDedicated != 2 { t.Fatalf("raw counts before=%d/%d preview=%d/%d after=%d/%d rerun=%d/%d", beforeGeneric, beforeDedicated, previewGeneric, previewDedicated, afterGeneric, afterDedicated, rerunGeneric, rerunDedicated) }
	requireMigratedToken(t, client, first)
	requireMigratedToken(t, client, second)
	t.Log("preview copy rerun authoritative load")
}

func TestDedicatedMigration_conversion_failure_rolls_back_and_preserves_sources(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	seedGenericToken(t, client, genericToken("good-before-bad", "valid-password"))
	bad, err := client.Resource.Create().SetUID("bad-source").SetName("bad-source").SetAPIVersion("v1").SetKind("Token").SetResourceType("Token").SetSpec([]byte(` + "`{}`" + `)).Save(t.Context())
	if err != nil { t.Fatal(err) }
	if bad.UID != "bad-source" { t.Fatal("bad source seed failed") }
	if _, err := db.ExecContext(t.Context(), "UPDATE resources SET spec = ? WHERE uid = ?", []byte(` + "`{\"displayName\":`" + `), "bad-source"); err != nil { t.Fatal(err) }
	sourceSnapshot := snapshotGenericSources(t, db)

	// When
	report, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	if err == nil { t.Fatal("expected conversion failure") }
	if report.Copied != 0 || report.NextAfterID != 0 || len(report.Failures) != 1 || report.Failures[0].SourceUID != "bad-source" { t.Fatalf("report=%#v err=%v", report, err) }
	if generic != 2 || dedicated != 0 { t.Fatalf("raw counts generic=%d dedicated=%d", generic, dedicated) }
	if !strings.Contains(err.Error(), "bad-source") { t.Fatalf("error lacks source UID: %v", err) }
	requireGenericSourcesUnchanged(t, "corruption failure", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("failure rollback source preserved")
}

func TestDedicatedMigration_existing_destination_is_skipped_without_overwrite(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	source := genericToken("stale-uid", "new-password")
	seedGenericToken(t, client, source)
	existing := genericToken("stale-uid", "old-password")
	existing.Metadata.Name = "authoritative-existing"
	create, err := ToEntToken(t.Context(), client, existing)
	if err != nil { t.Fatal(err) }
	if _, err := create.Save(t.Context()); err != nil { t.Fatal(err) }
	sourceSnapshot := snapshotGenericSources(t, db)

	// When
	preview, err := PreviewTokenMigration(t.Context(), DedicatedMigrationOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	report, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	stored, err := client.Token.Query().Where(token.UIDEQ("stale-uid")).Only(t.Context())
	if err != nil { t.Fatal(err) }
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	requireReport(t, preview, 1, 0, 0, 1)
	requireReport(t, report, 1, 0, 0, 1)
	if stored.Name != "authoritative-existing" { t.Fatalf("existing destination overwritten: %q", stored.Name) }
	if err := bcrypt.CompareHashAndPassword([]byte(stored.SpecPassword), []byte("old-password")); err != nil { t.Fatal(err) }
	if generic != 1 || dedicated != 1 { t.Fatalf("raw counts generic=%d dedicated=%d", generic, dedicated) }
	requireGenericSourcesUnchanged(t, "stale destination", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("stale destination skipped")
}

func TestDedicatedMigration_bounded_batch_and_cancellation(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	seedGenericToken(t, client, genericToken("bounded-1", "password-one"))
	seedGenericToken(t, client, genericToken("bounded-2", "password-two"))
	sourceSnapshot := snapshotGenericSources(t, db)

	// When
	preview, err := PreviewTokenMigration(t.Context(), DedicatedMigrationOptions{Limit: 1})
	if err != nil { t.Fatal(err) }
	nextPreview, err := PreviewTokenMigration(t.Context(), DedicatedMigrationOptions{Limit: 1, AfterID: preview.NextAfterID})
	if err != nil { t.Fatal(err) }
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report, cancelErr := MigrateTokenFromGeneric(ctx, DedicatedMigrationOptions{Limit: 1})
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	requireReport(t, preview, 1, 1, 0, 0)
	requireReport(t, nextPreview, 1, 1, 0, 0)
	if strings.Join(preview.SourceUIDs, ",") != "bounded-1" || strings.Join(nextPreview.SourceUIDs, ",") != "bounded-2" { t.Fatalf("bounded previews=%#v then %#v", preview, nextPreview) }
	if preview.NextAfterID == 0 || nextPreview.NextAfterID <= preview.NextAfterID { t.Fatalf("preview cursors=%d then %d", preview.NextAfterID, nextPreview.NextAfterID) }
	if !errors.Is(cancelErr, context.Canceled) { t.Fatalf("cancel error=%v report=%#v", cancelErr, report) }
	if report.Copied != 0 || report.NextAfterID != 0 || generic != 2 || dedicated != 0 { t.Fatalf("report=%#v raw=%d/%d", report, generic, dedicated) }
	requireGenericSourcesUnchanged(t, "pre-cancelled", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("bounded batch and cancellation")
}
`

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const dedicatedMigrationFailureRuntimeTests = `
	func TestDedicatedMigration_preview_hash_failure_preserves_source(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	seedGenericToken(t, client, genericToken("preview-hash-bad", strings.Repeat("x", 73)))
	sourceSnapshot := snapshotGenericSources(t, db)

	// When
	report, err := PreviewTokenMigration(t.Context(), DedicatedMigrationOptions{Limit: 10})
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	if err == nil { t.Fatal("expected preview bcrypt failure") }
	requireReport(t, report, 1, 0, 0, 0)
	if report.NextAfterID != 0 || len(report.Failures) != 1 || report.Failures[0].SourceUID != "preview-hash-bad" || report.Failures[0].Stage != "convert-dedicated" { t.Fatalf("report=%#v err=%v", report, err) }
	if generic != 1 || dedicated != 0 { t.Fatalf("raw counts generic=%d dedicated=%d", generic, dedicated) }
	requireGenericSourcesUnchanged(t, "hash preview failure", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("hash preview failure source preserved")
}

func TestDedicatedMigration_hash_failure_rolls_back_and_preserves_sources(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	seedGenericToken(t, client, genericToken("hash-good", "valid-password"))
	seedGenericToken(t, client, genericToken("hash-bad", strings.Repeat("x", 73)))
	sourceSnapshot := snapshotGenericSources(t, db)

	// When
	report, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	if err == nil { t.Fatal("expected bcrypt failure") }
	if report.Copied != 0 || report.NextAfterID != 0 || len(report.Failures) != 1 || report.Failures[0].SourceUID != "hash-bad" || report.Failures[0].Stage != "convert-dedicated" { t.Fatalf("report=%#v err=%v", report, err) }
	if generic != 2 || dedicated != 0 { t.Fatalf("raw counts generic=%d dedicated=%d", generic, dedicated) }
	requireGenericSourcesUnchanged(t, "hash failure", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("hash failure rollback source preserved")
}

func TestDedicatedMigration_constraint_failure_rolls_back_and_preserves_sources(t *testing.T) {
	// Given
	client, db := openMigrationDB(t)
	first := genericToken("constraint-first", "first-password")
	second := genericToken("constraint-second", "second-password")
	first.Spec.DisplayName = "duplicate-display"
	second.Spec.DisplayName = "duplicate-display"
	seedGenericToken(t, client, first)
	seedGenericToken(t, client, second)
	sourceSnapshot := snapshotGenericSources(t, db)

	// When
	report, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	if err == nil { t.Fatal("expected unique constraint failure") }
	if report.Copied != 0 || report.NextAfterID != 0 || len(report.Failures) != 1 || report.Failures[0].SourceUID != "constraint-second" || report.Failures[0].Stage != "save-dedicated" { t.Fatalf("report=%#v err=%v", report, err) }
	if generic != 2 || dedicated != 0 { t.Fatalf("raw counts generic=%d dedicated=%d", generic, dedicated) }
	requireGenericSourcesUnchanged(t, "constraint failure", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("constraint failure rollback source preserved")
}

func TestDedicatedMigration_commit_failure_resets_cursor_and_copied(t *testing.T) {
	// Given
	client, db := openCommitFailureMigrationDB(t)
	seedGenericToken(t, client, genericToken("commit-failure", "valid-password"))
	sourceSnapshot := snapshotGenericSources(t, db)

	// When
	report, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	generic, dedicated := rawMigrationCounts(t, db)

	// Then
	if err == nil || !strings.Contains(err.Error(), "injected commit failure") { t.Fatalf("commit error=%v report=%#v", err, report) }
	if report.Copied != 0 || report.NextAfterID != 0 { t.Fatalf("report=%#v", report) }
	if generic != 1 || dedicated != 0 { t.Fatalf("raw counts generic=%d dedicated=%d", generic, dedicated) }
	requireGenericSourcesUnchanged(t, "commit failure", sourceSnapshot, snapshotGenericSources(t, db))
	t.Log("commit failure rollback cursor preserved")
}
`

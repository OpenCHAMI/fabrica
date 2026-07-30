// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

//go:build integration

package integration

const generatedPostgresRuntimeContract = `func TestGeneratedPostgresRuntime_CRUD_security_and_constraints(t *testing.T) {
	h := newPostgresHarness(t)
	input := postgresToken("token-1", "unique-slug")
	plaintext := input.Spec.Password
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	entity, err := h.client.Token.Query().Where(enttoken.UIDEQ(input.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }
	if entity.SpecPassword == plaintext { t.Fatal("bcrypt field stored plaintext") }
	if err := bcrypt.CompareHashAndPassword([]byte(entity.SpecPassword), []byte(plaintext)); err != nil { t.Fatal(err) }
	loaded, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	if loaded.Spec.Password != "" { t.Fatalf("sensitive bcrypt field leaked: %q", loaded.Spec.Password) }
	input.Spec.Enabled, input.Spec.Retries, input.Spec.ImmutableCode = true, 9, "replacement"
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	input.Spec.Enabled, input.Spec.Retries = false, 0
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	updated, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	if updated.Spec.Enabled || updated.Spec.Retries != 0 || updated.Spec.ImmutableCode != "fixed" { t.Fatalf("updated spec=%#v", updated.Spec) }
	duplicate := postgresToken("token-2", "unique-slug")
	if err := SaveToken(t.Context(), duplicate); err == nil { t.Fatal("duplicate unique slug succeeded") }
	if err := DeleteToken(t.Context(), input.Metadata.UID); err != nil { t.Fatal(err) }
	if err := DeleteToken(t.Context(), input.Metadata.UID); !errors.Is(err, ErrNotFound) { t.Fatalf("missing delete=%v", err) }
	t.Log("CRUD unique immutable bcrypt redaction")
}

func TestGeneratedPostgresRuntime_full_envelope_reopen_and_mixed_routing(t *testing.T) {
	h := newPostgresHarness(t)
	want := postgresToken("token-envelope", "envelope-slug")
	if err := SaveToken(t.Context(), want); err != nil { t.Fatal(err) }
	widget := &v1.Widget{APIVersion: "v1", Kind: "Widget", Metadata: fabrica.Metadata{Name: "widget", UID: "widget-1", CreatedAt: want.Metadata.CreatedAt, UpdatedAt: want.Metadata.UpdatedAt}, Spec: v1.WidgetSpec{Value: "generic"}}
	if err := SaveWidget(t.Context(), widget); err != nil { t.Fatal(err) }
	var dedicated, generic int
	if err := h.db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM tokens").Scan(&dedicated); err != nil { t.Fatal(err) }
	if err := h.db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM resources").Scan(&generic); err != nil { t.Fatal(err) }
	if dedicated != 1 || generic != 1 { t.Fatalf("mixed counts dedicated=%d generic=%d", dedicated, generic) }
	want.Spec.Password = ""
	h.reopen(t)
	got, err := LoadToken(t.Context(), want.Metadata.UID)
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(got, want) { t.Fatalf("reopen got=%#v want=%#v", got, want) }
	loadedWidget, err := LoadWidget(t.Context(), widget.Metadata.UID)
	if err != nil || loadedWidget.Spec.Value != "generic" { t.Fatalf("widget=%#v err=%v", loadedWidget, err) }
	if _, err := h.db.ExecContext(t.Context(), "UPDATE tokens SET status=$1 WHERE uid=$2", []byte(` + "`{\"state\":\"corrupt\"}`" + `), want.Metadata.UID); err != nil { t.Fatal(err) }
	if corrupt, err := LoadToken(t.Context(), want.Metadata.UID); err == nil || corrupt != nil { t.Fatalf("corrupt status resource=%#v err=%v", corrupt, err) }
	t.Log("full envelope reopen mixed routing")
}

func TestGeneratedPostgresRuntime_redacted_and_status_updates_preserve_hidden_columns(t *testing.T) {
	h := newPostgresHarness(t)
	input := postgresToken("token-hidden", "hidden-slug")
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	var passwordBefore []byte
	if err := h.db.QueryRowContext(t.Context(), "SELECT spec_password FROM tokens WHERE uid=$1", input.Metadata.UID).Scan(&passwordBefore); err != nil { t.Fatal(err) }
	loaded, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	loaded.Spec.Retries = 11
	if err := SaveToken(t.Context(), loaded); err != nil { t.Fatal(err) }
	var passwordAfter []byte
	var retries int
	if err := h.db.QueryRowContext(t.Context(), "SELECT spec_password,spec_retries FROM tokens WHERE uid=$1", input.Metadata.UID).Scan(&passwordAfter, &retries); err != nil { t.Fatal(err) }
	if !bytes.Equal(passwordAfter, passwordBefore) || retries != 11 { t.Fatal("redacted update changed password or missed ordinary field") }
	loaded.Spec.Password = "replacement-password"
	if err := SaveToken(t.Context(), loaded); err != nil { t.Fatal(err) }
	if err := h.db.QueryRowContext(t.Context(), "SELECT spec_password FROM tokens WHERE uid=$1", input.Metadata.UID).Scan(&passwordAfter); err != nil { t.Fatal(err) }
	if bytes.Equal(passwordAfter, passwordBefore) { t.Fatal("explicit password replacement did not change hash") }
	if err := bcrypt.CompareHashAndPassword(passwordAfter, []byte("replacement-password")); err != nil { t.Fatal(err) }
	loaded.Status.State = "status-only"
	loaded.Metadata.ResourceVersion = "8"
	loaded.Metadata.UpdatedAt = loaded.Metadata.UpdatedAt.Add(time.Minute)
	if err := SaveTokenStatus(t.Context(), loaded); err != nil { t.Fatal(err) }
	var passwordStatus []byte
	if err := h.db.QueryRowContext(t.Context(), "SELECT spec_password FROM tokens WHERE uid=$1", input.Metadata.UID).Scan(&passwordStatus); err != nil { t.Fatal(err) }
	if !bytes.Equal(passwordStatus, passwordAfter) { t.Fatal("status update changed password") }
	t.Log("redacted status hidden preservation")
}

func TestGeneratedPostgresRuntime_explicit_migration_preview_copy_rerun(t *testing.T) {
	h := newPostgresHarness(t)
	source := postgresToken("migration-source", "migration-slug")
	spec, err := json.Marshal(source.Spec)
	if err != nil { t.Fatal(err) }
	status, err := json.Marshal(source.Status)
	if err != nil { t.Fatal(err) }
	if _, err := h.client.Resource.Create().SetUID(source.Metadata.UID).SetName(source.Metadata.Name).SetAPIVersion(source.APIVersion).SetKind(source.Kind).SetResourceType(source.Kind).SetSpec(spec).SetStatus(status).SetCreatedAt(source.Metadata.CreatedAt).SetUpdatedAt(source.Metadata.UpdatedAt).SetResourceVersion(source.Metadata.ResourceVersion).SetNamespace(source.Metadata.Namespace).Save(t.Context()); err != nil { t.Fatal(err) }
	count, err := h.client.Token.Query().Count(t.Context())
	if err != nil || count != 0 { t.Fatalf("migration ran automatically count=%d err=%v", count, err) }
	preview, err := PreviewTokenMigration(t.Context(), DedicatedMigrationOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	migrated, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	rerun, err := MigrateTokenFromGeneric(t.Context(), DedicatedMigrationOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	if preview.Scanned != 1 || preview.Copied != 0 || migrated.Copied != 1 || rerun.Skipped != 1 { t.Fatalf("preview=%#v migrated=%#v rerun=%#v", preview, migrated, rerun) }
	if preview.NextAfterID == 0 || migrated.NextAfterID != preview.NextAfterID || rerun.NextAfterID != preview.NextAfterID { t.Fatalf("commit-aware cursors preview=%d migrated=%d rerun=%d", preview.NextAfterID, migrated.NextAfterID, rerun.NextAfterID) }
	var generic int
	if err := h.db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM resources WHERE uid=$1", source.Metadata.UID).Scan(&generic); err != nil { t.Fatal(err) }
	if generic != 1 { t.Fatalf("migration removed generic source count=%d", generic) }
	loaded, err := LoadToken(t.Context(), source.Metadata.UID)
	if err != nil { t.Fatal(err) }
	if loaded.Metadata.Name != source.Metadata.Name || loaded.Spec.Password != "" { t.Fatalf("migrated token=%#v", loaded) }
	t.Log("explicit migration preview copy rerun")
}

`

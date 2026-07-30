// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const dedicatedSecurityRuntimeTest = `package storage

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage/ent"
	"example.com/generated-annotation-acceptance/internal/storage/ent/token"
	"github.com/openchami/fabrica/pkg/fabrica"
)

func openSecurityDB(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "security.db"))+"?_fk=1")
	if err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	return client, db
}

func securityToken(uid string) *v1.Token {
	stringValue := "pointer-string"
	boolValue := true
	intValue := 21
	int64Value := int64(22)
	floatValue := 23.5
	timeValue := time.Date(2026, 7, 20, 1, 2, 3, 0, time.UTC)
	return &v1.Token{
		APIVersion: "v1", Kind: "Token",
		Metadata: fabrica.Metadata{Name: uid, UID: uid},
		Spec: v1.TokenSpec{
			DisplayName: "visible", Password: "required-password",
			ImmutableSecret: "immutable-secret", SensitiveNote: "private-note",
			SensitiveBool: true, SensitiveInt: 11, SensitiveInt64: 12,
			SensitiveFloat64: 13.5, SensitiveTime: time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC),
			SensitiveStrings: []string{"initial"}, SensitiveStringPtr: &stringValue,
			SensitiveBoolPtr: &boolValue, SensitiveIntPtr: &intValue,
			SensitiveInt64Ptr: &int64Value, SensitiveFloat64Ptr: &floatValue,
			SensitiveTimePtr: &timeValue,
		},
	}
}

func saveSecurityToken(t *testing.T, client *ent.Client, input *v1.Token) *ent.Token {
	t.Helper()
	create, err := ToEntToken(t.Context(), client, input)
	if err != nil { t.Fatal(err) }
	entity, err := create.Save(t.Context())
	if err != nil { t.Fatal(err) }
	return entity
}

func requireBcrypt(t *testing.T, hash, plaintext string) {
	t.Helper()
	if hash == plaintext { t.Fatalf("stored plaintext %q", plaintext) }
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)); err != nil {
		t.Fatalf("bcrypt compare: %v", err)
	}
}

func TestDedicatedSecurity_required_bcrypt_create(t *testing.T) {
	client, _ := openSecurityDB(t)
	input := securityToken("required")
	entity := saveSecurityToken(t, client, input)
	requireBcrypt(t, entity.SpecPassword, input.Spec.Password)
	requireBcrypt(t, entity.SpecImmutableSecret, input.Spec.ImmutableSecret)
	empty := securityToken("empty-required")
	empty.Spec.Password = ""
	create, err := ToEntToken(t.Context(), client, empty)
	if err == nil || create != nil { t.Fatalf("empty required bcrypt: create=%#v err=%v", create, err) }
	t.Log("required bcrypt create")
}

func TestDedicatedSecurity_optional_bcrypt_omitted(t *testing.T) {
	client, db := openSecurityDB(t)
	input := securityToken("optional")
	saveSecurityToken(t, client, input)
	var present int
	if err := db.QueryRowContext(t.Context(), "SELECT spec_optional_key IS NOT NULL FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&present); err != nil { t.Fatal(err) }
	if present != 0 { t.Fatal("omitted optional credential was persisted") }
	t.Log("optional bcrypt omitted")
}

func TestDedicatedSecurity_mutable_bcrypt_update(t *testing.T) {
	client, _ := openSecurityDB(t)
	input := securityToken("mutable")
	input.Spec.OptionalKey = "existing-key"
	entity := saveSecurityToken(t, client, input)
	oldOptional := entity.SpecOptionalKey
	input.Spec.Password = "updated-password"
	input.Spec.OptionalKey = ""
	update := entity.Update()
	if err := UpdateTokenFromResource(t.Context(), update, input); err != nil { t.Fatal(err) }
	updated, err := update.Save(t.Context())
	if err != nil { t.Fatal(err) }
	requireBcrypt(t, updated.SpecPassword, input.Spec.Password)
	if updated.SpecOptionalKey != oldOptional { t.Fatal("empty optional credential changed stored hash") }
	t.Log("mutable bcrypt update")
}

func TestDedicatedSecurity_immutable_bcrypt_update_skipped(t *testing.T) {
	client, _ := openSecurityDB(t)
	input := securityToken("immutable")
	entity := saveSecurityToken(t, client, input)
	oldHash := entity.SpecImmutableSecret
	input.Spec.ImmutableSecret = "replacement-secret"
	update := entity.Update()
	if err := UpdateTokenFromResource(t.Context(), update, input); err != nil { t.Fatal(err) }
	updated, err := update.Save(t.Context())
	if err != nil { t.Fatal(err) }
	if updated.SpecImmutableSecret != oldHash { t.Fatal("immutable credential changed") }
	requireBcrypt(t, updated.SpecImmutableSecret, "immutable-secret")
	t.Log("immutable bcrypt skipped")
}

func TestDedicatedSecurity_sensitive_and_transformed_redacted(t *testing.T) {
	client, _ := openSecurityDB(t)
	input := securityToken("redacted")
	input.Spec.OptionalKey = "optional-key"
	entity := saveSecurityToken(t, client, input)
	got, err := FromEntToken(t.Context(), entity)
	if err != nil { t.Fatal(err) }
	if got.Spec.DisplayName != input.Spec.DisplayName { t.Fatalf("ordinary field = %q", got.Spec.DisplayName) }
	if got.Spec.Password != "" || got.Spec.OptionalKey != "" || got.Spec.ImmutableSecret != "" || got.Spec.SensitiveNote != "" {
		t.Fatalf("redaction failed: %#v", got.Spec)
	}
	if entity.SpecSensitiveNote != input.Spec.SensitiveNote { t.Fatal("ordinary sensitive field was not stored") }
	requireBcrypt(t, entity.SpecPassword, input.Spec.Password)
	t.Log("sensitive fields redacted")
}

func TestDedicatedSecurity_redacted_save_preserves_hidden_fields(t *testing.T) {
	// Given
	client, db := openSecurityDB(t)
	SetEntClient(client)
	input := securityToken("redacted-save")
	input.Spec.OptionalKey = "optional-key"
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	var passwordBefore, optionalBefore []byte
	var sensitiveBefore string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_optional_key,spec_sensitive_note FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordBefore, &optionalBefore, &sensitiveBefore); err != nil { t.Fatal(err) }
	loaded, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	loaded.Spec.DisplayName = "unrelated-update"

	// When
	err = SaveToken(t.Context(), loaded)

	// Then
	if err != nil { t.Fatal(err) }
	var passwordAfter, optionalAfter []byte
	var sensitiveAfter string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_optional_key,spec_sensitive_note FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordAfter, &optionalAfter, &sensitiveAfter); err != nil { t.Fatal(err) }
	if !bytes.Equal(passwordAfter, passwordBefore) || !bytes.Equal(optionalAfter, optionalBefore) || sensitiveAfter != sensitiveBefore { t.Fatal("redacted save changed hidden database bytes") }
	t.Log("redacted save preserves hidden fields")
}

func TestDedicatedSecurity_explicit_hidden_replacements(t *testing.T) {
	// Given
	client, db := openSecurityDB(t)
	SetEntClient(client)
	input := securityToken("hidden-replacement")
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	var passwordBefore []byte
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordBefore); err != nil { t.Fatal(err) }
	loaded, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	loaded.Spec.Password = "replacement-password"
	loaded.Spec.SensitiveNote = "replacement-note"

	// When
	err = SaveToken(t.Context(), loaded)

	// Then
	if err != nil { t.Fatal(err) }
	var passwordAfter []byte
	var sensitiveAfter string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_sensitive_note FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordAfter, &sensitiveAfter); err != nil { t.Fatal(err) }
	if bytes.Equal(passwordAfter, passwordBefore) { t.Fatal("password hash was not replaced") }
	requireBcrypt(t, string(passwordAfter), "replacement-password")
	if sensitiveAfter != "replacement-note" { t.Fatalf("sensitive replacement = %q", sensitiveAfter) }
	t.Log("explicit hidden replacements")
}

func TestDedicatedSecurity_status_only_persistence(t *testing.T) {
	// Given
	client, db := openSecurityDB(t)
	SetEntClient(client)
	input := securityToken("status-only")
	input.Metadata.UpdatedAt = time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	input.Metadata.ResourceVersion = "7"
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	var passwordBefore []byte
	var displayBefore, sensitiveBefore string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_display_name,spec_sensitive_note FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordBefore, &displayBefore, &sensitiveBefore); err != nil { t.Fatal(err) }
	loaded, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	loaded.Status.State = "ready"
	loaded.Metadata.UpdatedAt = loaded.Metadata.UpdatedAt.Add(time.Minute)
	loaded.Metadata.ResourceVersion = "8"

	// When
	err = SaveTokenStatus(t.Context(), loaded)

	// Then
	if err != nil { t.Fatal(err) }
	var passwordAfter []byte
	var displayAfter, sensitiveAfter, resourceVersion string
	var updatedAt time.Time
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_display_name,spec_sensitive_note,resource_version,updated_at FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordAfter, &displayAfter, &sensitiveAfter, &resourceVersion, &updatedAt); err != nil { t.Fatal(err) }
	if !bytes.Equal(passwordAfter, passwordBefore) || displayAfter != displayBefore || sensitiveAfter != sensitiveBefore { t.Fatal("status-only update changed spec") }
	if resourceVersion != "8" || !updatedAt.Equal(loaded.Metadata.UpdatedAt) { t.Fatalf("status metadata version=%q updated=%s", resourceVersion, updatedAt) }
	t.Log("status-only persistence")
}

func TestDedicatedSecurity_status_conversion_failure_has_no_write(t *testing.T) {
	// Given
	client, db := openSecurityDB(t)
	SetEntClient(client)
	input := securityToken("status-error")
	input.Status.State = "before"
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	var statusBefore []byte
	if err := db.QueryRowContext(t.Context(), "SELECT status FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&statusBefore); err != nil { t.Fatal(err) }
	input.Status = v1.TokenStatus{State: "after", FailMarshal: true}

	// When
	err := SaveTokenStatus(t.Context(), input)

	// Then
	if err == nil { t.Fatal("expected status conversion error") }
	var statusAfter []byte
	if err := db.QueryRowContext(t.Context(), "SELECT status FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&statusAfter); err != nil { t.Fatal(err) }
	if !bytes.Equal(statusAfter, statusBefore) { t.Fatal("status changed after conversion failure") }
	t.Log("status conversion failure no write")
}

func TestDedicatedSecurity_bcrypt_create_error_has_no_write(t *testing.T) {
	client, _ := openSecurityDB(t)
	input := securityToken("create-error")
	input.Spec.Password = strings.Repeat("x", 73)
	create, err := ToEntToken(t.Context(), client, input)
	if err == nil || create != nil { t.Fatalf("create=%#v err=%v", create, err) }
	count, countErr := client.Token.Query().Count(t.Context())
	if countErr != nil { t.Fatal(countErr) }
	if count != 0 { t.Fatalf("rows after hash error = %d", count) }
	t.Log("bcrypt create failure no write")
}

func TestDedicatedSecurity_bcrypt_update_error_has_no_write(t *testing.T) {
	client, _ := openSecurityDB(t)
	input := securityToken("update-error")
	entity := saveSecurityToken(t, client, input)
	oldHash := entity.SpecPassword
	input.Spec.Password = strings.Repeat("x", 73)
	input.Spec.DisplayName = "must-not-persist"
	err := UpdateTokenFromResource(t.Context(), entity.Update(), input)
	if err == nil { t.Fatal("expected bcrypt update error") }
	stored, queryErr := client.Token.Query().Where(token.UIDEQ(input.Metadata.UID)).Only(t.Context())
	if queryErr != nil { t.Fatal(queryErr) }
	if stored.SpecPassword != oldHash || stored.SpecDisplayName != "visible" { t.Fatal("database changed after update hash error") }
	t.Log("bcrypt update failure no write")
}
`

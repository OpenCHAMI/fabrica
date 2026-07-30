// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const dedicatedSecurityHandlerRuntimeTest = `package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	"github.com/go-chi/chi/v5"
	_ "github.com/mattn/go-sqlite3"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage"
	"example.com/generated-annotation-acceptance/internal/storage/ent"
	"github.com/openchami/fabrica/pkg/fabrica"
	"github.com/openchami/fabrica/pkg/resource"
)

type Config struct{}

var tokenPrefixOnce sync.Once

func registerTokenPrefix() {
	tokenPrefixOnce.Do(func() { resource.RegisterResourcePrefix("Token", "token") })
}

func openHandlerSecurityDB(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "handler.db"))+"?_fk=1")
	if err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	storage.SetEntClient(client)
	return client, db
}

func handlerSecurityToken(uid string) *v1.Token {
	return &v1.Token{
		APIVersion: "v1", Kind: "Token",
		Metadata: fabrica.Metadata{Name: uid, UID: uid},
		Spec: v1.TokenSpec{DisplayName: uid, Password: "required-password", ImmutableSecret: "immutable-secret", SensitiveNote: "private-note"},
	}
}

func requestWithUID(t *testing.T, method, path, uid, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/merge-patch+json")
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("uid", uid)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
}

func TestDedicatedSecurity_patch_without_credentials_preserves_database_bytes(t *testing.T) {
	// Given
	_, db := openHandlerSecurityDB(t)
	input := handlerSecurityToken("patch-hidden")
	if err := storage.SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	var passwordBefore []byte
	var sensitiveBefore string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_sensitive_note FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordBefore, &sensitiveBefore); err != nil { t.Fatal(err) }
	recorder := httptest.NewRecorder()
	req := requestWithUID(t, http.MethodPatch, "/tokens/"+input.Metadata.UID, input.Metadata.UID, ` + "`{\"displayName\":\"patched\"}`" + `)

	// When
	PatchToken(recorder, req)

	// Then
	if recorder.Code != http.StatusOK { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	var response v1.Token
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Spec.DisplayName != "patched" || response.Spec.Password != "" || response.Spec.SensitiveNote != "" { t.Fatalf("PATCH response leaked or missed persisted value: %#v", response.Spec) }
	var passwordAfter []byte
	var sensitiveAfter, displayAfter string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_sensitive_note,spec_display_name FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordAfter, &sensitiveAfter, &displayAfter); err != nil { t.Fatal(err) }
	if !bytes.Equal(passwordAfter, passwordBefore) || sensitiveAfter != sensitiveBefore || displayAfter != "patched" { t.Fatal("PATCH changed hidden fields or missed ordinary update") }
	t.Log("PATCH without credentials")
}

func TestDedicatedSecurity_create_response_reloads_redacted_persisted_resource(t *testing.T) {
	// Given
	openHandlerSecurityDB(t)
	registerTokenPrefix()
	body := ` + "`{\"metadata\":{\"name\":\"created\"},\"spec\":{\"displayName\":\"created-display\",\"password\":\"create-password\",\"immutableSecret\":\"create-immutable\",\"sensitiveNote\":\"create-note\"}}`" + `
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tokens", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	// When
	CreateToken(recorder, req)

	// Then
	if recorder.Code != http.StatusCreated { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	if bytes.Contains(recorder.Body.Bytes(), []byte("create-password")) || bytes.Contains(recorder.Body.Bytes(), []byte("create-note")) || bytes.Contains(recorder.Body.Bytes(), []byte("$2")) { t.Fatalf("create response leaked secret material: %s", recorder.Body.String()) }
	var response v1.Token
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Spec.DisplayName != "created-display" || response.Spec.Password != "" || response.Spec.SensitiveNote != "" { t.Fatalf("create response=%#v", response.Spec) }
	t.Log("create persisted redacted response")
}

func TestDedicatedSecurity_update_response_reloads_explicit_hidden_replacements(t *testing.T) {
	// Given
	openHandlerSecurityDB(t)
	input := handlerSecurityToken("update-response")
	if err := storage.SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	body := ` + "`{\"metadata\":{\"name\":\"update-response\"},\"spec\":{\"displayName\":\"updated-display\",\"password\":\"replacement-password\",\"sensitiveNote\":\"replacement-note\"}}`" + `
	recorder := httptest.NewRecorder()
	req := requestWithUID(t, http.MethodPut, "/tokens/"+input.Metadata.UID, input.Metadata.UID, body)

	// When
	UpdateToken(recorder, req)

	// Then
	if recorder.Code != http.StatusOK { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	if bytes.Contains(recorder.Body.Bytes(), []byte("replacement-password")) || bytes.Contains(recorder.Body.Bytes(), []byte("replacement-note")) || bytes.Contains(recorder.Body.Bytes(), []byte("$2")) { t.Fatalf("update response leaked secret material: %s", recorder.Body.String()) }
	var response v1.Token
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Spec.DisplayName != "updated-display" || response.Spec.Password != "" || response.Spec.SensitiveNote != "" { t.Fatalf("update response=%#v", response.Spec) }
	t.Log("update persisted redacted response")
}

func TestDedicatedSecurity_patch_response_reloads_explicit_hidden_replacements(t *testing.T) {
	// Given
	openHandlerSecurityDB(t)
	input := handlerSecurityToken("patch-response")
	if err := storage.SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	recorder := httptest.NewRecorder()
	req := requestWithUID(t, http.MethodPatch, "/tokens/"+input.Metadata.UID, input.Metadata.UID, ` + "`{\"displayName\":\"patched-display\",\"password\":\"patched-password\",\"sensitiveNote\":\"patched-note\"}`" + `)

	// When
	PatchToken(recorder, req)

	// Then
	if recorder.Code != http.StatusOK { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	if bytes.Contains(recorder.Body.Bytes(), []byte("patched-password")) || bytes.Contains(recorder.Body.Bytes(), []byte("patched-note")) || bytes.Contains(recorder.Body.Bytes(), []byte("$2")) { t.Fatalf("patch response leaked secret material: %s", recorder.Body.String()) }
	var response v1.Token
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Spec.DisplayName != "patched-display" || response.Spec.Password != "" || response.Spec.SensitiveNote != "" { t.Fatalf("patch response=%#v", response.Spec) }
	t.Log("patch persisted redacted response")
}

func TestDedicatedSecurity_patch_duplicate_returns_stable_conflict_without_mutation(t *testing.T) {
	// Given
	_, db := openHandlerSecurityDB(t)
	first := handlerSecurityToken("patch-conflict-first")
	second := handlerSecurityToken("patch-conflict-second")
	if err := storage.SaveToken(t.Context(), first); err != nil { t.Fatal(err) }
	if err := storage.SaveToken(t.Context(), second); err != nil { t.Fatal(err) }
	recorder := httptest.NewRecorder()
	req := requestWithUID(t, http.MethodPatch, "/tokens/"+second.Metadata.UID, second.Metadata.UID, ` + "`{\"displayName\":\"patch-conflict-first\"}`" + `)

	// When
	PatchToken(recorder, req)

	// Then
	if recorder.Code != http.StatusConflict { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Code != http.StatusConflict || response.Error != storage.ErrStorageConflict.Error() { t.Fatalf("response=%#v", response) }
	var displayName string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_display_name FROM tokens WHERE uid = ?", second.Metadata.UID).Scan(&displayName); err != nil { t.Fatal(err) }
	if displayName != second.Spec.DisplayName { t.Fatalf("displayName=%q, want %q", displayName, second.Spec.DisplayName) }
	t.Log("PATCH duplicate conflict")
}

func TestDedicatedSecurity_status_endpoint_updates_only_status(t *testing.T) {
	// Given
	_, db := openHandlerSecurityDB(t)
	input := handlerSecurityToken("status-handler")
	if err := storage.SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	var passwordBefore []byte
	var sensitiveBefore, displayBefore string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_sensitive_note,spec_display_name FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordBefore, &sensitiveBefore, &displayBefore); err != nil { t.Fatal(err) }
	recorder := httptest.NewRecorder()
	req := requestWithUID(t, http.MethodPut, "/tokens/"+input.Metadata.UID+"/status", input.Metadata.UID, ` + "`{\"state\":\"ready\"}`" + `)

	// When
	UpdateTokenStatus(recorder, req)

	// Then
	if recorder.Code != http.StatusOK { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	var passwordAfter []byte
	var sensitiveAfter, displayAfter, status string
	if err := db.QueryRowContext(t.Context(), "SELECT spec_password,spec_sensitive_note,spec_display_name,status FROM tokens WHERE uid = ?", input.Metadata.UID).Scan(&passwordAfter, &sensitiveAfter, &displayAfter, &status); err != nil { t.Fatal(err) }
	if !bytes.Equal(passwordAfter, passwordBefore) || sensitiveAfter != sensitiveBefore || displayAfter != displayBefore { t.Fatal("status endpoint changed spec") }
	if !bytes.Contains([]byte(status), []byte(` + "`\"state\":\"ready\"`" + `)) { t.Fatalf("status=%s", status) }
	t.Log("status endpoint update")
}
`

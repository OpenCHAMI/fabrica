// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const mixedStorageRuntimeTest = `package storage

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage/ent"
	enttoken "example.com/generated-annotation-acceptance/internal/storage/ent/token"
	"github.com/openchami/fabrica/pkg/fabrica"
)

func openMixedStorage(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:mixed?mode=memory&cache=shared&_fk=1")
	if err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	SetEntClient(client)
	return client, db
}

func mixedToken(uid, name string) *v1.Token {
	now := time.Date(2026, 7, 1, 2, 3, 4, 0, time.UTC)
	return &v1.Token{
		APIVersion: "acceptance.example.io/v1", Kind: "Token",
		Metadata: fabrica.Metadata{Name: name, UID: uid, Namespace: "tenant-a", CreatedAt: now, UpdatedAt: now, Labels: map[string]string{"environment": "test"}},
		Spec: v1.TokenSpec{DisplayName: "created"}, Status: v1.TokenStatus{State: "ready"},
	}
}

func mixedWidget() *v1.Widget {
	now := time.Date(2026, 7, 1, 2, 3, 4, 0, time.UTC)
	return &v1.Widget{APIVersion: "v1", Kind: "Widget", Metadata: fabrica.Metadata{Name: "generic", UID: "widget-1", CreatedAt: now, UpdatedAt: now}, Spec: v1.WidgetSpec{Value: "created"}}
}

func rawMixedTableCounts(t *testing.T, db *sql.DB) (int, int) {
	t.Helper()
	var tokens int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM tokens").Scan(&tokens); err != nil { t.Fatal(err) }
	var resources int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM resources").Scan(&resources); err != nil { t.Fatal(err) }
	return tokens, resources
}

func TestMixedStorage_create_update_delete_touch_one_table_family(t *testing.T) {
	// Given
	client, db := openMixedStorage(t)
	token := mixedToken("token-1", "dedicated")
	widget := mixedWidget()

	// When
	if err := SaveToken(t.Context(), token); err != nil { t.Fatal(err) }
	if err := SaveWidget(t.Context(), widget); err != nil { t.Fatal(err) }
	createdTokens, createdResources := rawMixedTableCounts(t, db)
	token.Spec.DisplayName = "updated"
	widget.Spec.Value = "updated"
	if err := SaveToken(t.Context(), token); err != nil { t.Fatal(err) }
	if err := SaveWidget(t.Context(), widget); err != nil { t.Fatal(err) }
	updatedTokens, updatedResources := rawMixedTableCounts(t, db)
	dedicatedCount, err := client.Token.Query().Count(t.Context())
	if err != nil { t.Fatal(err) }
	genericCount, err := client.Resource.Query().Count(t.Context())
	if err != nil { t.Fatal(err) }
	loadedToken, err := LoadToken(t.Context(), token.Metadata.UID)
	if err != nil { t.Fatal(err) }
	loadedWidget, err := LoadWidget(t.Context(), widget.Metadata.UID)
	if err != nil { t.Fatal(err) }

	// Then
	if createdTokens != 1 || createdResources != 1 { t.Fatalf("raw create tokens=%d resources=%d, want 1 each", createdTokens, createdResources) }
	if updatedTokens != 1 || updatedResources != 1 { t.Fatalf("raw update tokens=%d resources=%d, want 1 each", updatedTokens, updatedResources) }
	if dedicatedCount != 1 || genericCount != 1 { t.Fatalf("dedicated=%d generic=%d, want 1 each", dedicatedCount, genericCount) }
	if loadedToken.Spec.DisplayName != "updated" || loadedWidget.Spec.Value != "updated" { t.Fatalf("token=%#v widget=%#v", loadedToken.Spec, loadedWidget.Spec) }
	if !loadedToken.Metadata.CreatedAt.Equal(token.Metadata.CreatedAt) { t.Fatalf("dedicated createdAt changed: %s", loadedToken.Metadata.CreatedAt) }
	if err := DeleteToken(t.Context(), token.Metadata.UID); err != nil { t.Fatal(err) }
	if err := DeleteWidget(t.Context(), widget.Metadata.UID); err != nil { t.Fatal(err) }
	deletedTokens, deletedResources := rawMixedTableCounts(t, db)
	if deletedTokens != 0 || deletedResources != 0 { t.Fatalf("raw delete tokens=%d resources=%d, want 0 each", deletedTokens, deletedResources) }
	if count, err := client.Token.Query().Count(t.Context()); err != nil || count != 0 { t.Fatalf("dedicated after delete=%d err=%v", count, err) }
	if count, err := client.Resource.Query().Count(t.Context()); err != nil || count != 0 { t.Fatalf("generic after delete=%d err=%v", count, err) }
	if err := DeleteToken(t.Context(), token.Metadata.UID); !errors.Is(err, ErrNotFound) { t.Fatalf("dedicated missing delete=%v", err) }
	t.Log("exclusive create update delete")
}

func TestMixedStorage_dedicated_query_helpers_use_typed_table_and_metadata(t *testing.T) {
	// Given
	client, _ := openMixedStorage(t)
	input := mixedToken("token-query", "query-name")
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }

	// When
	var typedQuery *ent.TokenQuery = Querytokens(t.Context())
	count, err := typedQuery.Where(enttoken.NameEQ("query-name"), enttoken.NamespaceEQ("tenant-a")).Count(t.Context())
	if err != nil { t.Fatal(err) }
	byUID, err := GetTokenByUID(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	byName, err := GetTokenByName(t.Context(), "query-name", "tenant-a")
	if err != nil { t.Fatal(err) }
	byLabels, err := ListtokensByLabels(t.Context(), map[string]string{"environment": "test"})
	if err != nil { t.Fatal(err) }
	genericCount, err := client.Resource.Query().Count(t.Context())
	if err != nil { t.Fatal(err) }

	// Then
	if count != 1 || byUID.Metadata.UID != input.Metadata.UID || byName.Metadata.UID != input.Metadata.UID || len(byLabels) != 1 || genericCount != 0 {
		t.Fatalf("count=%d uid=%q name=%q labels=%d generic=%d", count, byUID.Metadata.UID, byName.Metadata.UID, len(byLabels), genericCount)
	}
	t.Log("dedicated query helpers")
}

func TestMixedStorage_dedicated_list_returns_no_partial_results_on_conversion_error(t *testing.T) {
	// Given
	_, db := openMixedStorage(t)
	if err := SaveToken(t.Context(), mixedToken("token-good", "good")); err != nil { t.Fatal(err) }
	if err := SaveToken(t.Context(), mixedToken("token-bad", "bad")); err != nil { t.Fatal(err) }
	if _, err := db.ExecContext(t.Context(), "UPDATE tokens SET status = ? WHERE uid = ?", []byte(` + "`{\"state\":\"corrupt\"}`" + `), "token-bad"); err != nil { t.Fatal(err) }

	// When
	items, err := LoadAllTokens(t.Context())

	// Then
	if err == nil { t.Fatal("expected conversion error") }
	if items != nil { t.Fatalf("conversion error returned partial items: %#v", items) }
	t.Log("conversion failure no partial list")
}
`

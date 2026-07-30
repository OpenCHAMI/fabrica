// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

//go:build integration

package integration

const generatedPostgresRuntimeCatalog = `type postgresColumn struct {
	Name string
	Type string
	Nullable string
	Default sql.NullString
}

func TestGeneratedPostgresRuntime_DDL_defaults_nullability_and_index_methods(t *testing.T) {
	h := newPostgresHarness(t)
	rows, err := h.db.QueryContext(t.Context(), ` + "`" + `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema=current_schema() AND table_name='tokens'
		ORDER BY ordinal_position
	` + "`" + `)
	if err != nil { t.Fatal(err) }
	defer rows.Close()
	columns := make(map[string]postgresColumn)
	for rows.Next() {
		var column postgresColumn
		if err := rows.Scan(&column.Name, &column.Type, &column.Nullable, &column.Default); err != nil { t.Fatal(err) }
		columns[column.Name] = column
	}
	if err := rows.Err(); err != nil { t.Fatal(err) }
	names := make([]string, 0, len(columns))
	for name := range columns { names = append(names, name) }
	sort.Strings(names)
	wantNames := []string{"annotations", "api_version", "created_at", "id", "kind", "labels", "name", "namespace", "resource_version", "spec_bucket", "spec_enabled", "spec_immutable_code", "spec_lookup", "spec_optional_note", "spec_password", "spec_retries", "spec_slug", "spec_tags", "status", "uid", "updated_at"}
	sort.Strings(wantNames)
	if !reflect.DeepEqual(names, wantNames) { t.Fatalf("columns=%v want=%v", names, wantNames) }
	for _, name := range []string{"uid", "name", "api_version", "kind", "created_at", "updated_at", "resource_version", "spec_lookup", "spec_slug", "spec_enabled", "spec_retries", "spec_immutable_code"} {
		if columns[name].Nullable != "NO" { t.Errorf("column %s nullable=%s", name, columns[name].Nullable) }
	}
	for _, name := range []string{"namespace", "status", "labels", "annotations", "spec_optional_note", "spec_tags", "spec_bucket", "spec_password"} {
		if columns[name].Nullable != "YES" { t.Errorf("column %s nullable=%s", name, columns[name].Nullable) }
	}
	defaultContains := map[string]string{"api_version": "v1", "kind": "Token", "resource_version": "1", "spec_enabled": "false", "spec_retries": "0", "spec_optional_note": "fallback"}
	for name, fragment := range defaultContains {
		if !columns[name].Default.Valid || !strings.Contains(columns[name].Default.String, fragment) { t.Errorf("column %s default=%#v want fragment %q", name, columns[name].Default, fragment) }
	}
	now := time.Date(2026, 7, 15, 1, 2, 3, 0, time.UTC)
	_, err = h.db.ExecContext(t.Context(), ` + "`" + `
		INSERT INTO tokens (uid,name,created_at,updated_at,spec_lookup,spec_slug,spec_immutable_code,spec_tags,spec_bucket,spec_password)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'[]'::jsonb,$8,$9)
	` + "`" + `, "raw-default", "raw-default", now, now, "lookup", "raw-default", "fixed", "bucket", "hash")
	if err != nil { t.Fatal(err) }
	var enabled bool
	var retries int
	var optional sql.NullString
	if err := h.db.QueryRowContext(t.Context(), "SELECT spec_enabled,spec_retries,spec_optional_note FROM tokens WHERE uid=$1", "raw-default").Scan(&enabled, &retries, &optional); err != nil { t.Fatal(err) }
	if enabled || retries != 0 || !optional.Valid || optional.String != "fallback" { t.Fatalf("defaults enabled=%t retries=%d optional=%#v", enabled, retries, optional) }
	_, err = h.db.ExecContext(t.Context(), ` + "`" + `
		INSERT INTO tokens (uid,name,created_at,updated_at,spec_lookup,spec_slug,spec_immutable_code,spec_tags,spec_bucket,spec_password,spec_enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'[]'::jsonb,$8,$9,NULL)
	` + "`" + `, "raw-null", "raw-null", now, now, "lookup-null", "raw-null", "fixed", "bucket", "hash")
	if err == nil { t.Fatal("explicit NULL scalar default succeeded") }
	if _, err := h.db.ExecContext(t.Context(), "UPDATE tokens SET spec_optional_note=NULL WHERE uid=$1", "raw-default"); err != nil { t.Fatalf("nullable pointer rejected NULL: %v", err) }

	indexRows, err := h.db.QueryContext(t.Context(), ` + "`" + `
		SELECT am.amname, pg_get_indexdef(index_class.oid)
		FROM pg_catalog.pg_class AS index_class
		JOIN pg_catalog.pg_index AS index_meta ON index_meta.indexrelid=index_class.oid
		JOIN pg_catalog.pg_class AS table_class ON table_class.oid=index_meta.indrelid
		JOIN pg_catalog.pg_namespace AS namespace ON namespace.oid=table_class.relnamespace
		JOIN pg_catalog.pg_am AS am ON am.oid=index_class.relam
		WHERE namespace.nspname=current_schema() AND table_class.relname='tokens'
		ORDER BY index_class.relname
	` + "`" + `)
	if err != nil { t.Fatal(err) }
	defer indexRows.Close()
	var indexes []string
	for indexRows.Next() {
		var method, definition string
		if err := indexRows.Scan(&method, &definition); err != nil { t.Fatal(err) }
		indexes = append(indexes, method+" "+definition)
	}
	if err := indexRows.Err(); err != nil { t.Fatal(err) }
	joined := strings.ToLower(strings.Join(indexes, "\n"))
	for _, expected := range []string{"btree create index", "spec_lookup", "btree create unique index", "spec_slug", "gin create index", "spec_tags", "hash create index", "spec_bucket"} {
		if !strings.Contains(joined, expected) { t.Errorf("indexes missing %q:\n%s", expected, joined) }
	}
	t.Logf("DDL defaults nullability indexes columns=%v indexes=%s", columns, joined)
}
`

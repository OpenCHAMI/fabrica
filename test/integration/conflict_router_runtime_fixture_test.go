// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const conflictRouterRuntimeTest = `package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/generated-annotation-acceptance/internal/storage"
)

func TestDedicatedSecurity_live_router_returns_stable_conflicts_without_mutation(t *testing.T) {
	tests := []struct {
		name, method, path, uid, body string
		seedSecond bool
	}{
		{name: "POST", method: http.MethodPost, path: "/tokens/", body: ` + "`{\"metadata\":{\"name\":\"new\"},\"spec\":{\"displayName\":\"conflict-first\",\"password\":\"password\",\"immutableSecret\":\"immutable\",\"sensitiveNote\":\"note\"}}`" + `},
		{name: "PUT", method: http.MethodPut, path: "/tokens/conflict-second/", uid: "conflict-second", body: ` + "`{\"spec\":{\"displayName\":\"conflict-first\"}}`" + `, seedSecond: true},
		{name: "PATCH", method: http.MethodPatch, path: "/tokens/conflict-second/", uid: "conflict-second", body: ` + "`{\"displayName\":\"conflict-first\"}`" + `, seedSecond: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			openHandlerSecurityDB(t)
			registerTokenPrefix()
			if err := storage.SaveToken(t.Context(), handlerSecurityToken("conflict-first")); err != nil { t.Fatal(err) }
			if test.seedSecond {
				if err := storage.SaveToken(t.Context(), handlerSecurityToken(test.uid)); err != nil { t.Fatal(err) }
			}
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/merge-patch+json")
			recorder := httptest.NewRecorder()
			router := liveRouterFixture{defaultLimit: DefaultRequestBodyMaxBytes}.build(t)

			// When
			router.ServeHTTP(recorder, request)

			// Then
			if recorder.Code != http.StatusConflict { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
			var response ErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatal(err) }
			if response.Error != storage.ErrStorageConflict.Error() || response.Code != http.StatusConflict { t.Fatalf("response=%#v", response) }
			all, err := storage.LoadAllTokens(t.Context())
			if err != nil { t.Fatal(err) }
			wantRows := 1
			if test.seedSecond { wantRows = 2 }
			if len(all) != wantRows { t.Fatalf("conflict changed row count=%d, want %d", len(all), wantRows) }
			if test.seedSecond {
				second, err := storage.LoadToken(t.Context(), test.uid)
				if err != nil { t.Fatal(err) }
				if second.Spec.DisplayName != test.uid { t.Fatalf("conflict mutated second resource: %#v", second.Spec) }
			}
		})
	}
	t.Log("live router conflicts")
}
`

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

const requestBodyHandlerRuntimeTest = `package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/generated-annotation-acceptance/internal/storage"
	"github.com/openchami/fabrica/pkg/httpbody"
)

type chunkReader struct { data []byte }

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 { return 0, io.EOF }
	if len(p) > 17 { p = p[:17] }
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

type bodyShape struct { prefix, suffix string }

func (s bodyShape) atSize(t *testing.T, size int) string {
	t.Helper()
	padding := size - len(s.prefix) - len(s.suffix)
	if padding < 0 { t.Fatalf("body framing exceeds target size %d", size) }
	return s.prefix + strings.Repeat("x", padding) + s.suffix
}

type bodyEndpointCase struct {
	name, method, path, uid string
	shape bodyShape
	invoke func(http.ResponseWriter, *http.Request)
	seed, streaming bool
}

func (c bodyEndpointCase) request(t *testing.T, body string) *http.Request {
	t.Helper()
	req := requestWithUID(t, c.method, c.path, c.uid, body)
	if c.streaming {
		req.Body = io.NopCloser(&chunkReader{data: []byte(body)})
		req.ContentLength = -1
	}
	return req
}

func TestDedicatedSecurity_body_reading_endpoints_reject_oversized_bodies_before_writes(t *testing.T) {
	const expectedBodyLimit = int(httpbody.DefaultMaxBytes)
	tests := []bodyEndpointCase{
		{name: "create declared length", method: http.MethodPost, path: "/tokens", shape: bodyShape{prefix: ` + "`{\"metadata\":{\"name\":\"large-create\"},\"spec\":{\"displayName\":\"large-create\",\"password\":\"password\",\"immutableSecret\":\"immutable\",\"sensitiveNote\":\"note\"},\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, invoke: CreateToken},
		{name: "update streaming", method: http.MethodPut, path: "/tokens/large-update", uid: "large-update", shape: bodyShape{prefix: ` + "`{\"spec\":{\"displayName\":\"updated\"},\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, invoke: UpdateToken, seed: true, streaming: true},
		{name: "patch declared length", method: http.MethodPatch, path: "/tokens/large-patch", uid: "large-patch", shape: bodyShape{prefix: ` + "`{\"displayName\":\"updated\",\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, invoke: PatchToken, seed: true},
		{name: "status update streaming", method: http.MethodPut, path: "/tokens/large-status/status", uid: "large-status", shape: bodyShape{prefix: ` + "`{\"state\":\"updated\",\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, invoke: UpdateTokenStatus, seed: true, streaming: true},
		{name: "status patch without content length", method: http.MethodPatch, path: "/tokens/large-status-patch/status", uid: "large-status-patch", shape: bodyShape{prefix: ` + "`{\"state\":\"updated\",\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, invoke: PatchTokenStatus, seed: true, streaming: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			openHandlerSecurityDB(t)
			if test.seed {
				if err := storage.SaveToken(t.Context(), handlerSecurityToken(test.uid)); err != nil { t.Fatal(err) }
			}
			body := test.shape.atSize(t, expectedBodyLimit+1)
			recorder := httptest.NewRecorder()
			req := test.request(t, body)

			// When
			test.invoke(recorder, req)

			// Then
			if recorder.Code != http.StatusRequestEntityTooLarge { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
			var response ErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatal(err) }
			if response.Code != http.StatusRequestEntityTooLarge || response.Error != "request body too large" { t.Fatalf("response=%#v", response) }
			if test.seed {
				loaded, err := storage.LoadToken(t.Context(), test.uid)
				if err != nil { t.Fatal(err) }
				if loaded.Spec.DisplayName != test.uid || loaded.Status.State != "" { t.Fatalf("oversized request mutated resource: %#v", loaded) }
			} else {
				all, err := storage.LoadAllTokens(t.Context())
				if err != nil { t.Fatal(err) }
				if len(all) != 0 { t.Fatalf("oversized create wrote %d resources", len(all)) }
			}
		})
	}
	t.Log("all body endpoints bounded")
}

func TestDedicatedSecurity_exact_body_limit_is_accepted(t *testing.T) {
	// Given
	const expectedBodyLimit = int(httpbody.DefaultMaxBytes)
	openHandlerSecurityDB(t)
	registerTokenPrefix()
	shape := bodyShape{prefix: ` + "`{\"metadata\":{\"name\":\"exact\"},\"spec\":{\"displayName\":\"exact\",\"password\":\"password\",\"immutableSecret\":\"immutable\",\"sensitiveNote\":\"note\"},\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}
	body := shape.atSize(t, expectedBodyLimit)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tokens", strings.NewReader(body))

	// When
	CreateToken(recorder, req)

	// Then
	if recorder.Code != http.StatusCreated { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	t.Log("exact body limit accepted")
}

func TestDedicatedSecurity_malformed_under_limit_keeps_existing_statuses_without_writes(t *testing.T) {
	tests := []struct {
		name, method string
		want, expectedRows int
		seed bool
		invoke func(http.ResponseWriter, *http.Request)
	}{
		{name: "create JSON decode", method: http.MethodPost, want: http.StatusBadRequest, invoke: CreateToken},
		{name: "patch document", method: http.MethodPatch, want: http.StatusUnprocessableEntity, expectedRows: 1, seed: true, invoke: PatchToken},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			openHandlerSecurityDB(t)
			uid := "malformed"
			if test.seed {
				if err := storage.SaveToken(t.Context(), handlerSecurityToken(uid)); err != nil { t.Fatal(err) }
			}
			recorder := httptest.NewRecorder()
			req := requestWithUID(t, test.method, "/tokens/"+uid, uid, "{")

			// When
			test.invoke(recorder, req)

			// Then
			if recorder.Code != test.want { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
			all, err := storage.LoadAllTokens(t.Context())
			if err != nil { t.Fatal(err) }
			if len(all) != test.expectedRows { t.Fatalf("malformed request changed storage: %#v", all) }
		})
	}
	t.Log("malformed under limit unchanged")
}
`

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

const requestBodyRouterRuntimeTest = `package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"example.com/generated-annotation-acceptance/internal/storage"
	"github.com/openchami/fabrica/pkg/versioning"
)

type liveRouterFixture struct {
	defaultLimit int64
	overrides map[string]int64
	afterLimit *int
}

func (f liveRouterFixture) build(t *testing.T) http.Handler {
	t.Helper()
	limitMiddleware, err := newRequestBodyLimitMiddleware(f.defaultLimit, f.overrides)
	if err != nil { t.Fatal(err) }
	registry := versioning.NewVersionRegistry()
	info := versioning.ResourceTypeInfo{Metadata: versioning.SchemaVersion{Version: "v1", IsDefault: true}}
	if err := registry.RegisterVersion("Token", "v1", info); err != nil { t.Fatal(err) }
	router := chi.NewRouter()
	router.Use(limitMiddleware)
	if f.afterLimit != nil {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				(*f.afterLimit)++
				next.ServeHTTP(w, r)
			})
		})
	}
	router.Use(versioning.VersionNegotiationMiddleware(registry, nil))
	RegisterGeneratedRoutes(router)
	return router
}

type liveBodyCase struct {
	name, method, path, uid, contentType string
	shape bodyShape
	seed, streaming bool
}

func (c liveBodyCase) request(t *testing.T, size int) *http.Request {
	t.Helper()
	body := c.shape.atSize(t, size)
	request := httptest.NewRequest(c.method, c.path, strings.NewReader(body))
	request.Header.Set("Content-Type", c.contentType)
	if c.streaming {
		request.Body = io.NopCloser(&chunkReader{data: []byte(body)})
		request.ContentLength = -1
	}
	return request
}

func TestDedicatedSecurity_live_router_bounds_all_body_paths_before_writes(t *testing.T) {
	const limit = 1024
	tests := []liveBodyCase{
		{name: "POST declared", method: http.MethodPost, path: "/tokens/", contentType: "application/json", shape: bodyShape{prefix: ` + "`{\"apiVersion\":\"v1\",\"metadata\":{\"name\":\"router-create\"},\"spec\":{\"displayName\":\"router-create\",\"password\":\"password\",\"immutableSecret\":\"immutable\",\"sensitiveNote\":\"note\"},\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}},
		{name: "PUT chunked", method: http.MethodPut, path: "/tokens/router-put/", uid: "router-put", contentType: "application/json", shape: bodyShape{prefix: ` + "`{\"spec\":{\"displayName\":\"changed\"},\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, seed: true, streaming: true},
		{name: "merge PATCH declared", method: http.MethodPatch, path: "/tokens/router-merge/", uid: "router-merge", contentType: "application/merge-patch+json", shape: bodyShape{prefix: ` + "`{\"displayName\":\"changed\",\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, seed: true},
		{name: "JSON PATCH chunked", method: http.MethodPatch, path: "/tokens/router-json-patch/", uid: "router-json-patch", contentType: "application/json-patch+json", shape: bodyShape{prefix: ` + "`[{\"op\":\"replace\",\"path\":\"/displayName\",\"value\":\"`" + `, suffix: ` + "`\"}]`" + `}, seed: true, streaming: true},
		{name: "status PUT declared", method: http.MethodPut, path: "/tokens/router-status-put/status/", uid: "router-status-put", contentType: "application/json", shape: bodyShape{prefix: ` + "`{\"state\":\"changed\",\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, seed: true},
		{name: "status PATCH chunked", method: http.MethodPatch, path: "/tokens/router-status-patch/status/", uid: "router-status-patch", contentType: "application/merge-patch+json", shape: bodyShape{prefix: ` + "`{\"state\":\"changed\",\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}, seed: true, streaming: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			openHandlerSecurityDB(t)
			registerTokenPrefix()
			if test.seed {
				if err := storage.SaveToken(t.Context(), handlerSecurityToken(test.uid)); err != nil { t.Fatal(err) }
			}
			afterLimit := 0
			router := liveRouterFixture{defaultLimit: limit, afterLimit: &afterLimit}.build(t)
			request := test.request(t, limit+1)
			recorder := httptest.NewRecorder()

			// When
			router.ServeHTTP(recorder, request)

			// Then
			assertStableRequestTooLarge(t, recorder)
			if !test.streaming && afterLimit != 0 { t.Fatalf("declared oversized body reached inner middleware %d times", afterLimit) }
			if test.streaming && afterLimit != 1 { t.Fatalf("chunked oversized body inner middleware calls=%d", afterLimit) }
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
	t.Log("live router body limits")
}

func TestDedicatedSecurity_live_router_accepts_exact_and_inventory_scale_defaults(t *testing.T) {
	tests := []struct { name string; limit, size int; streaming bool }{
		{name: "exact configured declared length", limit: 4096, size: 4096},
		{name: "exact configured chunked", limit: 4096, size: 4096, streaming: true},
		{name: "four MiB inventory payload under default", limit: int(DefaultRequestBodyMaxBytes), size: 4 << 20},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			openHandlerSecurityDB(t)
			registerTokenPrefix()
			shape := bodyShape{prefix: ` + "`{\"apiVersion\":\"v1\",\"metadata\":{\"name\":\"inventory-scale\"},\"spec\":{\"displayName\":\"inventory-scale\",\"password\":\"password\",\"immutableSecret\":\"immutable\",\"sensitiveNote\":\"note\",\"inventory\":[\"`" + `, suffix: ` + "`\"]}}`" + `}
			body := shape.atSize(t, test.size)
			request := httptest.NewRequest(http.MethodPost, "/tokens/", strings.NewReader(body))
			if test.streaming {
				request.Body = io.NopCloser(&chunkReader{data: []byte(body)})
				request.ContentLength = -1
			}
			recorder := httptest.NewRecorder()
			router := liveRouterFixture{defaultLimit: int64(test.limit)}.build(t)

			// When
			router.ServeHTTP(recorder, request)

			// Then
			if recorder.Code != http.StatusCreated { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
		})
	}
	t.Log("inventory-scale default accepted")
}

func TestDedicatedSecurity_live_router_rejects_trailing_bodies_without_writes(t *testing.T) {
	tests := []struct { name, suffix string; limit int; want int }{
		{name: "huge trailing whitespace", suffix: strings.Repeat(" ", 2048), limit: 1024, want: http.StatusRequestEntityTooLarge},
		{name: "second JSON value", suffix: ` + "` {\"second\":true}`" + `, limit: 4096, want: http.StatusBadRequest},
		{name: "malformed trailing bytes", suffix: " garbage", limit: 4096, want: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			openHandlerSecurityDB(t)
			registerTokenPrefix()
			body := ` + "`{\"apiVersion\":\"v1\",\"metadata\":{\"name\":\"trailing\"},\"spec\":{\"displayName\":\"trailing\",\"password\":\"password\",\"immutableSecret\":\"immutable\",\"sensitiveNote\":\"note\"}}`" + ` + test.suffix
			request := httptest.NewRequest(http.MethodPost, "/tokens/", strings.NewReader(body))
			request.ContentLength = -1
			recorder := httptest.NewRecorder()
			router := liveRouterFixture{defaultLimit: int64(test.limit)}.build(t)

			// When
			router.ServeHTTP(recorder, request)

			// Then
			if recorder.Code != test.want { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
			all, err := storage.LoadAllTokens(t.Context())
			if err != nil { t.Fatal(err) }
			if len(all) != 0 { t.Fatalf("trailing body wrote %d resources", len(all)) }
		})
	}
	t.Log("live router trailing body contracts")
}

func TestRequestBodyLimitConfigurationRejectsInvalidValues(t *testing.T) {
	tests := []struct { name string; global int64; overrides map[string]int64 }{
		{name: "zero global", global: 0},
		{name: "negative global", global: -1},
		{name: "zero resource", global: 1024, overrides: map[string]int64{"Token": 0}},
		{name: "unknown resource", global: 1024, overrides: map[string]int64{"Unknown": 512}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newRequestBodyLimitMiddleware(test.global, test.overrides); err == nil {
				t.Fatal("invalid request body limit configuration succeeded")
			}
		})
	}
}

func TestDedicatedSecurity_live_router_honors_per_resource_override(t *testing.T) {
	// Given
	openHandlerSecurityDB(t)
	registerTokenPrefix()
	shape := bodyShape{prefix: ` + "`{\"metadata\":{\"name\":\"override\"},\"spec\":{\"displayName\":\"override\",\"password\":\"password\",\"immutableSecret\":\"immutable\",\"sensitiveNote\":\"note\"},\"padding\":\"`" + `, suffix: ` + "`\"}`" + `}
	body := shape.atSize(t, 1025)
	request := httptest.NewRequest(http.MethodPost, "/tokens/", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	router := liveRouterFixture{defaultLimit: 4096, overrides: map[string]int64{"Token": 1024}}.build(t)

	// When
	router.ServeHTTP(recorder, request)

	// Then
	assertStableRequestTooLarge(t, recorder)
}

func assertStableRequestTooLarge(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusRequestEntityTooLarge { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Error != "request body too large" || response.Code != http.StatusRequestEntityTooLarge { t.Fatalf("response=%#v", response) }
}

`

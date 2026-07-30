// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const generatedHandlerHooksRuntimeTest = `package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage"
	"github.com/openchami/fabrica/pkg/fabrica"
	fabricaResource "github.com/openchami/fabrica/pkg/resource"
	fabricaStorage "github.com/openchami/fabrica/pkg/storage"
)

var registerGeneratedTokenPrefix sync.Once

func resetGeneratedHandlerHooks(t *testing.T) {
	t.Helper()
	tokenHooks = TokenHooks{}
	backend, err := fabricaStorage.NewFileBackend(t.TempDir())
	if err != nil { t.Fatal(err) }
	storage.Init(backend)
	registerGeneratedTokenPrefix.Do(func() { fabricaResource.RegisterResourcePrefix("Token", "token") })
}

func createGeneratedToken(t *testing.T, value string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(` + "`" + `{"metadata":{"name":"test"},"spec":{"Value":"` + "`" + `+value+` + "`" + `"}}` + "`" + `)
	req := httptest.NewRequest(http.MethodPost, "/tokens", body)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	CreateToken(response, req)
	return response
}

func storedGeneratedTokens(t *testing.T) []*v1.Token {
	t.Helper()
	tokens, err := storage.LoadAllTokens(t.Context())
	if err != nil { t.Fatal(err) }
	return tokens
}

func TestGeneratedHandlerHooks_zero_value_preserves_generated_create(t *testing.T) {
	resetGeneratedHandlerHooks(t)

	response := createGeneratedToken(t, "original")

	if response.Code != http.StatusCreated { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
	tokens := storedGeneratedTokens(t)
	if len(tokens) != 1 || tokens[0].Spec.Value != "original" { t.Fatalf("stored=%#v", tokens) }
	t.Log("generated handler hook runtime")
}

func TestGeneratedHandlerHooks_guard_rejects_without_mutation(t *testing.T) {
	resetGeneratedHandlerHooks(t)
	cause := errors.New("private guard cause")
	tokenHooks.BeforeCreate = func(_ context.Context, _ *http.Request, _ *CreateTokenRequest) error {
		return &HandlerError{StatusCode: http.StatusForbidden, PublicMessage: "denied", Cause: cause}
	}

	response := createGeneratedToken(t, "blocked")

	if response.Code != http.StatusForbidden { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
	if strings.Contains(response.Body.String(), cause.Error()) { t.Fatalf("response leaked cause: %s", response.Body.String()) }
	if len(storedGeneratedTokens(t)) != 0 { t.Fatal("guard rejection mutated storage") }
	if !errors.Is(&HandlerError{Cause: cause}, cause) { t.Fatal("HandlerError lost cause chain") }
}

func TestGeneratedHandlerHooks_response_transforms_output_and_header_only(t *testing.T) {
	resetGeneratedHandlerHooks(t)
	tokenHooks.AfterCreate = func(_ context.Context, _ *http.Request, header http.Header, token *v1.Token) (*v1.Token, error) {
		header.Set("X-Hook", "applied")
		transformed := *token
		transformed.Spec.Value = "redacted"
		return &transformed, nil
	}

	response := createGeneratedToken(t, "stored")

	if response.Code != http.StatusCreated || response.Header().Get("X-Hook") != "applied" { t.Fatalf("response=%#v", response) }
	if !strings.Contains(response.Body.String(), "redacted") { t.Fatalf("body=%s", response.Body.String()) }
	tokens := storedGeneratedTokens(t)
	if len(tokens) != 1 || tokens[0].Spec.Value != "stored" { t.Fatalf("stored=%#v", tokens) }
}

func TestGeneratedHandlerHooks_executor_unhandled_uses_generated_storage(t *testing.T) {
	resetGeneratedHandlerHooks(t)
	called := false
	tokenHooks.ExecuteCreate = func(_ context.Context, _ *http.Request, _ *CreateTokenRequest) (*v1.Token, bool, error) {
		called = true
		return nil, false, nil
	}

	response := createGeneratedToken(t, "generated")

	if !called || response.Code != http.StatusCreated { t.Fatalf("called=%v status=%d", called, response.Code) }
	if len(storedGeneratedTokens(t)) != 1 { t.Fatal("unhandled executor bypassed generated storage") }
}

func TestGeneratedHandlerHooks_executor_handled_bypasses_generated_storage(t *testing.T) {
	resetGeneratedHandlerHooks(t)
	tokenHooks.ExecuteCreate = func(_ context.Context, _ *http.Request, _ *CreateTokenRequest) (*v1.Token, bool, error) {
		return &v1.Token{
			APIVersion: "acceptance.example.io/v1", Kind: "Token",
			Metadata: fabrica.Metadata{Name: "hook", UID: "hook-1"},
			Spec: v1.TokenSpec{Value: "authoritative"},
		}, true, nil
	}

	response := createGeneratedToken(t, "ignored")

	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), "authoritative") { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
	if len(storedGeneratedTokens(t)) != 0 { t.Fatal("handled executor used generated storage") }
}

func TestGeneratedHandlerHooks_handled_nil_result_is_safe_error(t *testing.T) {
	resetGeneratedHandlerHooks(t)
	tokenHooks.ExecuteCreate = func(_ context.Context, _ *http.Request, _ *CreateTokenRequest) (*v1.Token, bool, error) {
		return nil, true, nil
	}

	response := createGeneratedToken(t, "ignored")

	if response.Code != http.StatusInternalServerError { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
	if len(storedGeneratedTokens(t)) != 0 { t.Fatal("nil handled result mutated storage") }
}

func TestGeneratedHandlerHooks_response_error_does_not_rollback_mutation(t *testing.T) {
	resetGeneratedHandlerHooks(t)
	tokenHooks.AfterCreate = func(_ context.Context, _ *http.Request, _ http.Header, _ *v1.Token) (*v1.Token, error) {
		return nil, &HandlerError{StatusCode: http.StatusUnprocessableEntity, PublicMessage: "response rejected", Cause: errors.New("private response cause")}
	}

	response := createGeneratedToken(t, "persisted")

	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "response rejected") { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
	if len(storedGeneratedTokens(t)) != 1 { t.Fatal("response hook error rolled back generated mutation") }
}

func TestGeneratedHandlerHooks_invalid_typed_status_maps_to_500_without_cause(t *testing.T) {
	resetGeneratedHandlerHooks(t)
	tokenHooks.BeforeCreate = func(_ context.Context, _ *http.Request, _ *CreateTokenRequest) error {
		return &HandlerError{StatusCode: 99, PublicMessage: "safe public message", Cause: errors.New("private typed cause")}
	}

	response := createGeneratedToken(t, "blocked")

	if response.Code != http.StatusInternalServerError { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
	if !strings.Contains(response.Body.String(), "safe public message") || strings.Contains(response.Body.String(), "private typed cause") { t.Fatalf("body=%s", response.Body.String()) }
}
`

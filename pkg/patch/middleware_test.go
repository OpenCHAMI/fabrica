// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package patch

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPatchMiddlewareReturnsStable413BeforeSaveWhenBoundedBodyExceedsLimit(t *testing.T) {
	// Given
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/resources/one", bytes.NewBuffer(bytes.Repeat([]byte("x"), 65)))
	request.Body = http.MaxBytesReader(recorder, request.Body, 64)
	saves := 0
	handler := &PatchHandler{
		GetResource:  func(*http.Request) ([]byte, error) { return []byte(`{"value":"old"}`), nil },
		SaveResource: func(*http.Request, []byte) error { saves++; return nil },
	}

	// When
	handler.ServeHTTP(recorder, request)

	// Then
	assertStableRequestTooLarge(t, recorder)
	if saves != 0 {
		t.Fatalf("oversized patch performed %d saves", saves)
	}
}

func TestAutoPatchMiddlewareReturnsStable413BeforeDownstreamWhenBoundedBodyExceedsLimit(t *testing.T) {
	// Given
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/resources/one", bytes.NewBuffer(bytes.Repeat([]byte("x"), 65)))
	request.Body = http.MaxBytesReader(recorder, request.Body, 64)
	calls := 0
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ })

	// When
	AutoPatchMiddleware("resource")(next).ServeHTTP(recorder, request)

	// Then
	assertStableRequestTooLarge(t, recorder)
	if calls != 0 {
		t.Fatalf("oversized automatic patch made %d downstream calls", calls)
	}
}

func assertStableRequestTooLarge(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "request body too large" || response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("response=%#v", response)
	}
}

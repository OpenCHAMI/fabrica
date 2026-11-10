// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package core

import (
	"encoding/json"
	"net/http"
	"strings"
)

// TestCreateRejectsInvalidSpec covers that every validate tag is enforced on
// create with 400 and that nothing is stored.
func (s *CoreSuite) TestCreateRejectsInvalidSpec() {
	s.forEachKind(func(k resourceKind) {
		for name, mutate := range k.InvalidSpecs {
			s.Run(name, func() {
				id := uniqueID()
				spec := k.NewSpec(id)
				mutate(spec)
				resourceName := "invalid-" + id
				status, body := s.doJSON(http.MethodPost, k.Path, createBody(resourceName, spec, nil))
				s.Equal(http.StatusBadRequest, status, "expected 400: %s", body)
				s.assertErrorBody(body)
				for _, it := range s.list(k) {
					s.NotEqual(resourceName, it.Metadata.Name, "rejected %s was stored", k.Kind)
				}
			})
		}
	})
}

// TestUpdateRejectsInvalidSpec covers that PUT enforces the same validation
// and leaves the stored resource unchanged.
func (s *CoreSuite) TestUpdateRejectsInvalidSpec() {
	s.forEachKind(func(k resourceKind) {
		for name, mutate := range k.InvalidSpecs {
			s.Run(name, func() {
				created, spec := s.create(k)
				bad := cloneSpec(spec)
				mutate(bad)
				status, body := s.doJSON(http.MethodPut, k.Path+"/"+created.Metadata.UID, createBody(created.Metadata.Name, bad, nil))
				s.Equal(http.StatusBadRequest, status, "expected 400: %s", body)
				s.Equal(normalize(spec), any(s.get(k, created.Metadata.UID).Spec))
			})
		}
	})
}

// TestCreateRequiresSpec covers that a create request without a spec is
// rejected.
func (s *CoreSuite) TestCreateRequiresSpec() {
	s.forEachKind(func(k resourceKind) {
		status, body := s.doJSON(http.MethodPost, k.Path, map[string]any{"metadata": map[string]any{"name": "nospec-" + uniqueID()}})
		s.Equal(http.StatusBadRequest, status, "expected 400: %s", body)
	})
}

// TestMalformedJSON covers that malformed request bodies are rejected with 400
// on create and update.
func (s *CoreSuite) TestMalformedJSON() {
	s.forEachKind(func(k resourceKind) {
		created, _ := s.create(k)
		headers := map[string]string{"Content-Type": "application/json"}
		for _, tc := range []struct{ method, path string }{
			{http.MethodPost, k.Path},
			{http.MethodPut, k.Path + "/" + created.Metadata.UID},
		} {
			resp, body := s.do(tc.method, tc.path, []byte(`{"metadata": {"name": "x"`), headers)
			s.Equal(http.StatusBadRequest, resp.StatusCode, "%s %s: %s", tc.method, tc.path, body)
			s.assertErrorBody(body)
		}
	})
}

// TestUnknownUID covers that every item endpoint returns 404 for a UID that
// does not exist.
func (s *CoreSuite) TestUnknownUID() {
	s.forEachKind(func(k resourceKind) {
		uid := k.UIDPrefix + "doesnotexist"
		item := k.Path + "/" + uid
		spec := k.NewSpec(uniqueID())
		for _, tc := range []struct {
			method, path, contentType string
			payload                   any
		}{
			{http.MethodGet, item, "", nil},
			{http.MethodPut, item, "application/json", createBody("x", spec, nil)},
			{http.MethodPatch, item, mergePatchType, map[string]any{"model": "x"}},
			{http.MethodDelete, item, "", nil},
			{http.MethodPut, item + "/status", "application/json", k.NewStatus()},
			{http.MethodPatch, item + "/status", mergePatchType, map[string]any{"phase": "x"}},
		} {
			var data []byte
			if tc.payload != nil {
				data, _ = json.Marshal(tc.payload)
			}
			resp, body := s.do(tc.method, tc.path, data, map[string]string{"Content-Type": tc.contentType})
			s.Equal(http.StatusNotFound, resp.StatusCode, "%s %s: %s", tc.method, tc.path, body)
		}
	})
}

// TestUnsupportedMethod covers that methods without a route are rejected.
func (s *CoreSuite) TestUnsupportedMethod() {
	s.forEachKind(func(k resourceKind) {
		status, body := s.doJSON(http.MethodDelete, k.Path, nil)
		s.Equal(http.StatusMethodNotAllowed, status, "DELETE %s: %s", k.Path, body)
		status, body = s.doJSON(http.MethodPut, k.Path, map[string]any{})
		s.Equal(http.StatusMethodNotAllowed, status, "PUT %s: %s", k.Path, body)
	})
}

// assertErrorBody checks the generated JSON error response shape.
func (s *CoreSuite) assertErrorBody(body []byte) {
	var e struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}
	if s.NoError(json.Unmarshal(body, &e), "error body is not JSON: %s", body) {
		s.NotEmpty(strings.TrimSpace(e.Error), "error body has no error message: %s", body)
		s.Equal(http.StatusBadRequest, e.Code, "error body: %s", body)
	}
}

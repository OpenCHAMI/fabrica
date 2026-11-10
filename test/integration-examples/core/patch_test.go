// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package core

import (
	"encoding/json"
	"net/http"
)

const (
	mergePatchType     = "application/merge-patch+json"
	jsonPatchType      = "application/json-patch+json"
	shorthandPatchType = "application/shorthand-patch+json"
)

// patch sends a PATCH with the given content type and returns status and body.
func (s *CoreSuite) patch(path, contentType string, payload any) (int, []byte) {
	data, err := json.Marshal(payload)
	s.Require().NoError(err)
	resp, body := s.do(http.MethodPatch, path, data, map[string]string{"Content-Type": contentType})
	return resp.StatusCode, body
}

// TestPatchMerge covers PATCH /<resources>/{uid} with a JSON merge patch
// (RFC 7386): patched fields change and untouched fields, nested siblings, and
// existing map keys are preserved.
func (s *CoreSuite) TestPatchMerge() {
	s.forEachKind(func(k resourceKind) {
		created, _ := s.create(k)
		status, body := s.patch(k.Path+"/"+created.Metadata.UID, mergePatchType, k.MergePatch)
		s.Require().Equal(http.StatusOK, status, "merge patch: %s", body)
		k.CheckMergePatch(s, s.decode(body).Spec)

		got := s.get(k, created.Metadata.UID)
		k.CheckMergePatch(s, got.Spec)
		s.Equal(created.Metadata.Name, got.Metadata.Name)
	})
}

// TestPatchDefaultsToMerge covers that a PATCH sent as plain application/json
// is treated as a merge patch.
func (s *CoreSuite) TestPatchDefaultsToMerge() {
	s.forEachKind(func(k resourceKind) {
		created, _ := s.create(k)
		status, body := s.patch(k.Path+"/"+created.Metadata.UID, "application/json", k.MergePatch)
		s.Require().Equal(http.StatusOK, status, "patch: %s", body)
		k.CheckMergePatch(s, s.get(k, created.Metadata.UID).Spec)
	})
}

// TestPatchMergeNullRemovesField covers that null in a merge patch removes the
// field.
func (s *CoreSuite) TestPatchMergeNullRemovesField() {
	k := nodeKind()
	created, _ := s.create(k)
	status, body := s.patch(k.Path+"/"+created.Metadata.UID, mergePatchType, map[string]any{
		"redfishPath": nil,
		"properties":  map[string]any{"bios": nil},
	})
	s.Require().Equal(http.StatusOK, status, "merge patch: %s", body)

	got := s.get(k, created.Metadata.UID)
	s.Empty(got.Spec["redfishPath"])
	props := asMap(s, got.Spec["properties"])
	s.NotContains(props, "bios")
	s.Equal("2", props["cpus"])
}

// TestPatchJSONPatch covers PATCH with a JSON Patch (RFC 6902) document using
// replace, add, and remove operations.
func (s *CoreSuite) TestPatchJSONPatch() {
	s.Run("Node", func() {
		k := nodeKind()
		created, _ := s.create(k)
		ops := []map[string]any{
			{"op": "test", "path": "/nodeType", "value": "Compute"},
			{"op": "replace", "path": "/nodeType", "value": "Management"},
			{"op": "replace", "path": "/location/rack", "value": "R42"},
			{"op": "add", "path": "/childrenUIDs/-", "value": "node-appended"},
			{"op": "add", "path": "/properties/gpu", "value": "4"},
			{"op": "remove", "path": "/properties/cpus"},
		}
		status, body := s.patch(k.Path+"/"+created.Metadata.UID, jsonPatchType, ops)
		s.Require().Equal(http.StatusOK, status, "json patch: %s", body)

		got := s.get(k, created.Metadata.UID)
		s.Equal("Management", got.Spec["nodeType"])
		s.Equal("R42", asMap(s, got.Spec["location"])["rack"])
		children, _ := got.Spec["childrenUIDs"].([]any)
		s.Len(children, 3)
		if len(children) == 3 {
			s.Equal("node-appended", children[2])
		}
		props := asMap(s, got.Spec["properties"])
		s.Equal("4", props["gpu"])
		s.NotContains(props, "cpus")
	})
	s.Run("User", func() {
		k := userKind()
		created, _ := s.create(k)
		ops := []map[string]any{
			{"op": "replace", "path": "/role", "value": "admin"},
			{"op": "replace", "path": "/active", "value": false},
			{"op": "remove", "path": "/apiKey"},
		}
		status, body := s.patch(k.Path+"/"+created.Metadata.UID, jsonPatchType, ops)
		s.Require().Equal(http.StatusOK, status, "json patch: %s", body)

		got := s.get(k, created.Metadata.UID)
		s.Equal("admin", got.Spec["role"])
		s.Equal(false, got.Spec["active"])
		s.Empty(got.Spec["apiKey"])
	})
}

// TestPatchShorthand covers PATCH with the shorthand format
// ({"dotted.path": value}).
func (s *CoreSuite) TestPatchShorthand() {
	k := nodeKind()
	created, _ := s.create(k)
	status, body := s.patch(k.Path+"/"+created.Metadata.UID, shorthandPatchType, map[string]any{
		"model":         "XD9000",
		"location.slot": "S11",
	})
	s.Require().Equal(http.StatusOK, status, "shorthand patch: %s", body)

	got := s.get(k, created.Metadata.UID)
	s.Equal("XD9000", got.Spec["model"])
	loc := asMap(s, got.Spec["location"])
	s.Equal("S11", loc["slot"])
	s.Equal("R1", loc["rack"])
}

// TestPatchFailedTestOp covers that a JSON Patch whose test operation fails is
// rejected with 422 and leaves the resource unchanged.
func (s *CoreSuite) TestPatchFailedTestOp() {
	s.forEachKind(func(k resourceKind) {
		created, spec := s.create(k)
		firstField := "nodeType"
		if k.Kind == "User" {
			firstField = "role"
		}
		ops := []map[string]any{
			{"op": "test", "path": "/" + firstField, "value": "no-such-value"},
			{"op": "replace", "path": "/" + firstField, "value": "changed"},
		}
		status, body := s.patch(k.Path+"/"+created.Metadata.UID, jsonPatchType, ops)
		s.Equal(http.StatusUnprocessableEntity, status, "failed test op: %s", body)
		s.Equal(normalize(spec), any(s.get(k, created.Metadata.UID).Spec))
	})
}

// TestPatchInvalidResult covers that a patch producing an invalid spec is
// rejected with 400 and leaves the resource unchanged.
func (s *CoreSuite) TestPatchInvalidResult() {
	cases := map[string]map[string]any{
		"Node": {"nodeType": ""},
		"User": {"role": "superuser"},
	}
	s.forEachKind(func(k resourceKind) {
		created, spec := s.create(k)
		status, body := s.patch(k.Path+"/"+created.Metadata.UID, mergePatchType, cases[k.Kind])
		s.Equal(http.StatusBadRequest, status, "invalid patch result: %s", body)
		s.Equal(normalize(spec), any(s.get(k, created.Metadata.UID).Spec))
	})
}

// TestPatchCannotChangeMetadataOrStatus covers that PATCH only applies to the
// spec: envelope-shaped keys are not applied to metadata or status.
func (s *CoreSuite) TestPatchCannotChangeMetadataOrStatus() {
	k := nodeKind()
	created, _ := s.create(k)
	status, body := s.patch(k.Path+"/"+created.Metadata.UID, mergePatchType, map[string]any{
		"metadata": map[string]any{"name": "hijacked", "uid": "node-hijacked"},
		"status":   map[string]any{"health": "Critical"},
	})
	s.Require().Equal(http.StatusOK, status, "patch: %s", body)

	got := s.get(k, created.Metadata.UID)
	s.Equal(created.Metadata.Name, got.Metadata.Name)
	s.Equal(created.Metadata.UID, got.Metadata.UID)
	s.Empty(got.Status["health"])
}

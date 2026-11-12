// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package core

import "net/http"

// TestStatusUpdate covers PUT /<resources>/{uid}/status: the status is
// replaced, round-trips exactly, and the spec and metadata are unchanged.
func (s *CoreSuite) TestStatusUpdate() {
	s.forEachKind(func(k resourceKind) {
		created, spec := s.create(k)
		uid := created.Metadata.UID
		st := k.NewStatus()

		status, body := s.doJSON(http.MethodPut, k.Path+"/"+uid+"/status", st)
		s.Require().Equal(http.StatusOK, status, "status update: %s", body)
		s.Equal(normalize(st), any(s.decode(body).Status))

		got := s.get(k, uid)
		s.Equal(normalize(st), any(got.Status))
		s.Equal(normalize(spec), any(got.Spec), "status update must not change the spec")
		s.Equal(created.Metadata.Name, got.Metadata.Name)
		s.True(created.Metadata.CreatedAt.Equal(got.Metadata.CreatedAt))
	})
}

// TestStatusUpdateReplaces covers that PUT status replaces the whole status.
func (s *CoreSuite) TestStatusUpdateReplaces() {
	k := nodeKind()
	created, _ := s.create(k)
	uid := created.Metadata.UID
	status, body := s.doJSON(http.MethodPut, k.Path+"/"+uid+"/status", k.NewStatus())
	s.Require().Equal(http.StatusOK, status, "status update: %s", body)

	status, body = s.doJSON(http.MethodPut, k.Path+"/"+uid+"/status", map[string]any{"health": "Warning"})
	s.Require().Equal(http.StatusOK, status, "status update: %s", body)
	s.Equal(map[string]any{"health": "Warning"}, s.get(k, uid).Status)
}

// TestStatusPatch covers PATCH /<resources>/{uid}/status with merge and JSON
// patches: patched fields change, other status fields and the spec are kept.
func (s *CoreSuite) TestStatusPatch() {
	s.Run("Node/merge", func() {
		k := nodeKind()
		created, spec := s.create(k)
		uid := created.Metadata.UID
		status, body := s.doJSON(http.MethodPut, k.Path+"/"+uid+"/status", k.NewStatus())
		s.Require().Equal(http.StatusOK, status, "status update: %s", body)

		status, body = s.patch(k.Path+"/"+uid+"/status", mergePatchType, map[string]any{
			"health":  "Critical",
			"metrics": map[string]any{"fanRPM": 9000},
		})
		s.Require().Equal(http.StatusOK, status, "status patch: %s", body)

		got := s.get(k, uid)
		s.Equal("Critical", got.Status["health"])
		s.Equal("Present", got.Status["state"])
		metrics := asMap(s, got.Status["metrics"])
		s.Equal(float64(9000), metrics["fanRPM"])
		s.Equal(0.75, metrics["cpuUtil"])
		s.Equal(normalize(spec), any(got.Spec))
	})
	s.Run("User/json-patch", func() {
		k := userKind()
		created, spec := s.create(k)
		uid := created.Metadata.UID
		status, body := s.patch(k.Path+"/"+uid+"/status", jsonPatchType, []map[string]any{
			{"op": "add", "path": "/phase", "value": "Locked"},
			{"op": "add", "path": "/message", "value": "too many failed logins"},
		})
		s.Require().Equal(http.StatusOK, status, "status patch: %s", body)

		got := s.get(k, uid)
		s.Equal("Locked", got.Status["phase"])
		s.Equal("too many failed logins", got.Status["message"])
		s.Equal(normalize(spec), any(got.Spec))
	})
}

// TestStatusMalformed covers that a malformed status body is rejected with 400.
func (s *CoreSuite) TestStatusMalformed() {
	s.forEachKind(func(k resourceKind) {
		created, _ := s.create(k)
		resp, body := s.do(http.MethodPut, k.Path+"/"+created.Metadata.UID+"/status", []byte("{not json"), map[string]string{"Content-Type": "application/json"})
		s.Equal(http.StatusBadRequest, resp.StatusCode, "malformed status: %s", body)
	})
}

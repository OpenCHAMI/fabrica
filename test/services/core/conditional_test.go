// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package core

import (
	"encoding/json"
	"net/http"
)

// TestETagIfNoneMatch covers ETags on GET of an item and of the list: a
// matching If-None-Match returns 304, and the ETag changes after an update.
func (s *CoreSuite) TestETagIfNoneMatch() {
	s.forEachKind(func(k resourceKind) {
		created, spec := s.create(k)
		item := k.Path + "/" + created.Metadata.UID

		resp, body := s.do(http.MethodGet, item, nil, nil)
		s.Require().Equal(http.StatusOK, resp.StatusCode, "get: %s", body)
		etag := resp.Header.Get("ETag")
		s.Require().NotEmpty(etag, "GET %s returned no ETag", item)

		resp, body = s.do(http.MethodGet, item, nil, map[string]string{"If-None-Match": etag})
		s.Equal(http.StatusNotModified, resp.StatusCode, "If-None-Match: %s", body)
		s.Empty(body)

		resp, _ = s.do(http.MethodGet, k.Path, nil, nil)
		s.Require().Equal(http.StatusOK, resp.StatusCode)
		listETag := resp.Header.Get("ETag")
		s.Require().NotEmpty(listETag, "list %s returned no ETag", k.Path)
		resp, _ = s.do(http.MethodGet, k.Path, nil, map[string]string{"If-None-Match": listETag})
		s.Equal(http.StatusNotModified, resp.StatusCode, "list If-None-Match")

		newSpec := cloneSpec(spec)
		k.Mutate(newSpec)
		status, body := s.doJSON(http.MethodPut, item, createBody(created.Metadata.Name, newSpec, nil))
		s.Require().Equal(http.StatusOK, status, "update: %s", body)

		resp, body = s.do(http.MethodGet, item, nil, map[string]string{"If-None-Match": etag})
		s.Equal(http.StatusOK, resp.StatusCode, "stale If-None-Match should return the resource: %s", body)
		s.NotEqual(etag, resp.Header.Get("ETag"), "ETag did not change after update")
	})
}

// TestIfMatch covers optimistic concurrency on every mutating item endpoint:
// a stale If-Match is rejected with 412 and changes nothing, and the current
// ETag is accepted.
func (s *CoreSuite) TestIfMatch() {
	s.forEachKind(func(k resourceKind) {
		created, spec := s.create(k)
		item := k.Path + "/" + created.Metadata.UID
		stale := `"stale-etag"`

		newSpec := cloneSpec(spec)
		k.Mutate(newSpec)
		putBody, _ := json.Marshal(createBody(created.Metadata.Name, newSpec, nil))
		patchBody, _ := json.Marshal(k.MergePatch)
		statusBody, _ := json.Marshal(k.NewStatus())

		requests := []struct {
			name, method, path, contentType string
			data                            []byte
		}{
			{"PUT", http.MethodPut, item, "application/json", putBody},
			{"PATCH", http.MethodPatch, item, mergePatchType, patchBody},
			{"PUT status", http.MethodPut, item + "/status", "application/json", statusBody},
			{"PATCH status", http.MethodPatch, item + "/status", mergePatchType, statusBody},
			{"DELETE", http.MethodDelete, item, "", nil},
		}

		for _, r := range requests {
			resp, body := s.do(r.method, r.path, r.data, map[string]string{"Content-Type": r.contentType, "If-Match": stale})
			s.Equal(http.StatusPreconditionFailed, resp.StatusCode, "%s with stale If-Match: %s", r.name, body)
		}
		s.Equal(normalize(spec), any(s.get(k, created.Metadata.UID).Spec), "stale If-Match requests changed the resource")

		for _, r := range requests {
			resp, _ := s.do(http.MethodGet, item, nil, nil)
			s.Require().Equal(http.StatusOK, resp.StatusCode)
			etag := resp.Header.Get("ETag")
			resp, body := s.do(r.method, r.path, r.data, map[string]string{"Content-Type": r.contentType, "If-Match": etag})
			s.Equal(http.StatusOK, resp.StatusCode, "%s with current If-Match: %s", r.name, body)
		}
		status, _ := s.doJSON(http.MethodGet, item, nil)
		s.Equal(http.StatusNotFound, status, "DELETE with current If-Match did not delete")
	})
}

// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package core

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// TestCreate covers POST /<resources>: the response is 201 with a generated
// UID, the envelope fields, the request metadata, and the spec echoed back.
func (s *CoreSuite) TestCreate() {
	s.forEachKind(func(k resourceKind) {
		id := uniqueID()
		spec := k.NewSpec(id)
		name := strings.ToLower(k.Kind) + "-" + id
		body := createBody(name, spec, map[string]string{"env": "test", "team": "hpc"})
		body["metadata"].(map[string]any)["annotations"] = map[string]string{"note": "created by test"}
		body["labels"] = map[string]string{"extra": "top-level"}

		status, resp := s.doJSON(http.MethodPost, k.Path, body)
		s.Require().Equal(http.StatusCreated, status, "create: %s", resp)
		got := s.decode(resp)

		s.True(strings.HasPrefix(got.Metadata.UID, k.UIDPrefix), "uid %q should start with %q", got.Metadata.UID, k.UIDPrefix)
		s.Equal(k.Kind, got.Kind)
		s.Equal(apiVersion, got.APIVersion)
		s.Equal(name, got.Metadata.Name)
		s.Equal(map[string]string{"env": "test", "team": "hpc", "extra": "top-level"}, got.Metadata.Labels)
		s.Equal("created by test", got.Metadata.Annotations["note"])
		s.False(got.Metadata.CreatedAt.IsZero(), "createdAt should be set")
		s.False(got.Metadata.UpdatedAt.IsZero(), "updatedAt should be set")
		s.Equal(normalize(spec), any(got.Spec))
	})
}

// TestCreateAssignsUniqueUIDs covers creating resources with the same name:
// each gets its own UID (names are not unique without annotations).
func (s *CoreSuite) TestCreateAssignsUniqueUIDs() {
	s.forEachKind(func(k resourceKind) {
		id := uniqueID()
		name := strings.ToLower(k.Kind) + "-dup-" + id
		var uids []string
		for i := 0; i < 2; i++ {
			status, body := s.doJSON(http.MethodPost, k.Path, createBody(name, k.NewSpec(id), nil))
			s.Require().Equal(http.StatusCreated, status, "create #%d: %s", i, body)
			uids = append(uids, s.decode(body).Metadata.UID)
		}
		s.NotEqual(uids[0], uids[1])
	})
}

// TestGet covers GET /<resources>/{uid}: the stored resource round-trips
// exactly, including nested objects, slices, maps, booleans, and timestamps.
func (s *CoreSuite) TestGet() {
	s.forEachKind(func(k resourceKind) {
		created, spec := s.create(k)
		got := s.get(k, created.Metadata.UID)

		s.Equal(created.Metadata.UID, got.Metadata.UID)
		s.Equal(created.Metadata.Name, got.Metadata.Name)
		s.Equal(k.Kind, got.Kind)
		s.Equal(apiVersion, got.APIVersion)
		s.Equal(normalize(spec), any(got.Spec))
		s.True(created.Metadata.CreatedAt.Equal(got.Metadata.CreatedAt), "createdAt changed: %v -> %v", created.Metadata.CreatedAt, got.Metadata.CreatedAt)
	})
}

// TestList covers GET /<resources>: all created resources are listed, and
// only resources of that kind are returned.
func (s *CoreSuite) TestList() {
	created := map[string][]string{}
	for _, k := range s.kinds {
		for i := 0; i < 3; i++ {
			obj, _ := s.create(k)
			created[k.Kind] = append(created[k.Kind], obj.Metadata.UID)
		}
	}
	s.forEachKind(func(k resourceKind) {
		items := s.list(k)
		for _, uid := range created[k.Kind] {
			s.True(containsUID(items, uid), "%s %s not found in list", k.Kind, uid)
		}
		for _, it := range items {
			s.Equal(k.Kind, it.Kind, "list of %s returned %s %s", k.Path, it.Kind, it.Metadata.UID)
		}
	})
}

// TestUpdate covers PUT /<resources>/{uid}: the spec is replaced, the name can
// be changed, labels are merged, and the UID and createdAt are preserved.
func (s *CoreSuite) TestUpdate() {
	s.forEachKind(func(k resourceKind) {
		id := uniqueID()
		spec := k.NewSpec(id)
		status, body := s.doJSON(http.MethodPost, k.Path, createBody("before-"+id, spec, map[string]string{"keep": "yes"}))
		s.Require().Equal(http.StatusCreated, status, "create: %s", body)
		created := s.decode(body)

		newSpec := cloneSpec(spec)
		k.Mutate(newSpec)
		update := createBody("after-"+id, newSpec, nil)
		update["labels"] = map[string]string{"added": "yes"}
		status, body = s.doJSON(http.MethodPut, k.Path+"/"+created.Metadata.UID, update)
		s.Require().Equal(http.StatusOK, status, "update: %s", body)
		updated := s.decode(body)
		s.Equal(normalize(newSpec), any(updated.Spec))

		got := s.get(k, created.Metadata.UID)
		s.Equal(created.Metadata.UID, got.Metadata.UID)
		s.Equal("after-"+id, got.Metadata.Name)
		s.Equal(normalize(newSpec), any(got.Spec))
		s.Equal(map[string]string{"keep": "yes", "added": "yes"}, got.Metadata.Labels)
		s.True(created.Metadata.CreatedAt.Equal(got.Metadata.CreatedAt), "createdAt changed on update")
		s.False(got.Metadata.UpdatedAt.Before(created.Metadata.UpdatedAt), "updatedAt went backwards")
	})
}

// TestUpdateReplacesWholeSpec covers PUT semantics: fields omitted from the
// new spec are cleared rather than merged.
func (s *CoreSuite) TestUpdateReplacesWholeSpec() {
	s.Run("Node", func() {
		k := nodeKind()
		created, spec := s.create(k)
		newSpec := map[string]any{"nodeType": "Storage"}
		status, body := s.doJSON(http.MethodPut, k.Path+"/"+created.Metadata.UID, createBody("", newSpec, nil))
		s.Require().Equal(http.StatusOK, status, "update: %s", body)

		got := s.get(k, created.Metadata.UID)
		s.Equal(created.Metadata.Name, got.Metadata.Name, "empty name in PUT must keep the existing name")
		s.Equal("Storage", got.Spec["nodeType"])
		s.Empty(got.Spec["serialNumber"], "serialNumber %v should be cleared", spec["serialNumber"])
		s.Empty(got.Spec["childrenUIDs"])
		s.Empty(got.Spec["properties"])
		s.Empty(got.Spec["redfishPath"])
	})
}

// TestUpdatePreservesStatus covers PUT of the spec not touching status.
func (s *CoreSuite) TestUpdatePreservesStatus() {
	s.forEachKind(func(k resourceKind) {
		created, spec := s.create(k)
		uid := created.Metadata.UID
		statusBody := k.NewStatus()
		status, body := s.doJSON(http.MethodPut, k.Path+"/"+uid+"/status", statusBody)
		s.Require().Equal(http.StatusOK, status, "status update: %s", body)

		newSpec := cloneSpec(spec)
		k.Mutate(newSpec)
		status, body = s.doJSON(http.MethodPut, k.Path+"/"+uid, createBody(created.Metadata.Name, newSpec, nil))
		s.Require().Equal(http.StatusOK, status, "update: %s", body)

		s.Equal(normalize(statusBody), any(s.get(k, uid).Status))
	})
}

// TestDelete covers DELETE /<resources>/{uid}: the response names the UID,
// the resource is gone from GET and list, and a second delete is 404.
func (s *CoreSuite) TestDelete() {
	s.forEachKind(func(k resourceKind) {
		created, _ := s.create(k)
		keep, _ := s.create(k)
		uid := created.Metadata.UID

		status, body := s.doJSON(http.MethodDelete, k.Path+"/"+uid, nil)
		s.Require().Equal(http.StatusOK, status, "delete: %s", body)
		var del struct {
			Message string `json:"message"`
			UID     string `json:"uid"`
		}
		s.Require().NoError(json.Unmarshal(body, &del), "body: %s", body)
		s.Equal(uid, del.UID)
		s.NotEmpty(del.Message)

		status, body = s.doJSON(http.MethodGet, k.Path+"/"+uid, nil)
		s.Equal(http.StatusNotFound, status, "GET after delete: %s", body)

		items := s.list(k)
		s.False(containsUID(items, uid), "deleted %s %s still listed", k.Kind, uid)
		s.True(containsUID(items, keep.Metadata.UID), "delete removed an unrelated %s", k.Kind)

		status, body = s.doJSON(http.MethodDelete, k.Path+"/"+uid, nil)
		s.Equal(http.StatusNotFound, status, "second delete: %s", body)
	})
}

// TestKindsAreIsolated covers that a UID of one kind is not reachable through
// another kind's endpoints.
func (s *CoreSuite) TestKindsAreIsolated() {
	node, _ := s.create(nodeKind())
	user, _ := s.create(userKind())

	status, body := s.doJSON(http.MethodGet, "/users/"+node.Metadata.UID, nil)
	s.Equal(http.StatusNotFound, status, "node uid via /users: %s", body)
	status, body = s.doJSON(http.MethodGet, "/nodes/"+user.Metadata.UID, nil)
	s.Equal(http.StatusNotFound, status, "user uid via /nodes: %s", body)
	status, body = s.doJSON(http.MethodDelete, "/users/"+node.Metadata.UID, nil)
	s.Equal(http.StatusNotFound, status, "delete node uid via /users: %s", body)
	s.get(nodeKind(), node.Metadata.UID)
}

// TestConcurrentCreates covers parallel POSTs: every request succeeds with a
// distinct UID and every resource is listed afterwards.
func (s *CoreSuite) TestConcurrentCreates() {
	const n = 20
	s.forEachKind(func(k resourceKind) {
		type result struct {
			status int
			body   []byte
		}
		results := make([]result, n)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				id := uniqueID()
				data, _ := json.Marshal(createBody("conc-"+id, k.NewSpec(id), nil))
				resp, err := s.client.Post(s.baseURL+k.Path, "application/json", bytes.NewReader(data))
				if err != nil {
					results[i] = result{status: -1, body: []byte(err.Error())}
					return
				}
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				results[i] = result{status: resp.StatusCode, body: body}
			}(i)
		}
		wg.Wait()

		uids := map[string]bool{}
		for i, r := range results {
			if !s.Equal(http.StatusCreated, r.status, "concurrent create #%d: %s", i, r.body) {
				continue
			}
			uids[s.decode(r.body).Metadata.UID] = true
		}
		s.Len(uids, n, "concurrent creates should yield %d distinct UIDs", n)
		items := s.list(k)
		for uid := range uids {
			s.True(containsUID(items, uid), "%s %s not listed", k.Kind, uid)
		}
	})
}

// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package core tests a two-resource service (Node and User, see
// resource-defs/) that is generated once per storage backend (file, sqlite,
// postgres) and run under fabrun. The same suite runs unchanged against every
// backend, so any backend-specific behavior shows up as a test failure.
//
// The Makefile in this directory generates, starts, and stops each service;
// `make test-only-<backend>` runs these tests against an already running
// service. Without FABRICA_EXAMPLE_URL the tests are skipped.
//
// Tests are grouped by file:
//
//	service_test.go     public service endpoints (health, status, catalog, OpenAPI, docs)
//	crud_test.go        create, get, list, update (PUT), delete for every resource
//	patch_test.go       PATCH of the spec with merge, JSON, and shorthand patches
//	status_test.go      PUT/PATCH of the status subresource
//	validation_test.go  request and spec validation, malformed input, unknown UIDs
//	conditional_test.go ETag, If-None-Match, and If-Match handling
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	apiGroup = "core.openchami.org"
	// apiVersion is the served version; fabrica currently returns the bare
	// version rather than "<group>/<version>".
	apiVersion = "v1"
)

// resourceObj is the generic envelope every resource is returned in.
type resourceObj struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   metadata       `json:"metadata"`
	Spec       map[string]any `json:"spec"`
	Status     map[string]any `json:"status"`
}

type metadata struct {
	UID         string            `json:"uid"`
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// resourceKind describes one resource type so the CRUD-style tests can run the
// same checks against every resource.
type resourceKind struct {
	Kind      string
	Path      string
	UIDPrefix string
	// NewSpec returns a fully populated, valid spec unique to id.
	NewSpec func(id string) map[string]any
	// Mutate changes the given valid spec into a different valid spec.
	Mutate func(spec map[string]any)
	// MergePatch is a merge patch for a top-level and a nested/map field; Check
	// asserts it was applied to the returned spec.
	MergePatch      map[string]any
	CheckMergePatch func(s *CoreSuite, spec map[string]any)
	// InvalidSpecs are specs (built from a valid one) the service must reject.
	InvalidSpecs map[string]func(spec map[string]any)
	// NewStatus returns a fully populated status.
	NewStatus func() map[string]any
}

type CoreSuite struct {
	suite.Suite
	baseURL string
	backend string
	client  *http.Client
	kinds   []resourceKind
}

func TestCore(t *testing.T) {
	suite.Run(t, new(CoreSuite))
}

func (s *CoreSuite) SetupSuite() {
	s.baseURL = strings.TrimRight(os.Getenv("FABRICA_EXAMPLE_URL"), "/")
	if s.baseURL == "" {
		s.T().Skip("FABRICA_EXAMPLE_URL is not set; run via 'make test-<backend>' or 'make up-<backend> test-only-<backend>'")
	}
	s.backend = os.Getenv("FABRICA_EXAMPLE_BACKEND")
	s.T().Logf("testing %s (backend %q)", s.baseURL, s.backend)
	s.client = &http.Client{Timeout: 15 * time.Second}
	s.kinds = []resourceKind{nodeKind(), userKind()}
	s.Require().NoError(s.waitForHealth(60*time.Second), "service at %s is not healthy; is it running ('make up-<backend>')?", s.baseURL)
}

// forEachKind runs fn as a subtest per resource kind.
func (s *CoreSuite) forEachKind(fn func(k resourceKind)) {
	for _, k := range s.kinds {
		s.Run(k.Kind, func() { fn(k) })
	}
}

// --- resource kinds --------------------------------------------------------

func nodeKind() resourceKind {
	return resourceKind{
		Kind:      "Node",
		Path:      "/nodes",
		UIDPrefix: "node-",
		NewSpec: func(id string) map[string]any {
			return map[string]any{
				"nodeType":     "Compute",
				"serialNumber": "SN-" + id,
				"partNumber":   "PN-1000",
				"manufacturer": "HPE",
				"model":        "XD2000",
				"location": map[string]any{
					"bmcUID":  "bmc-" + id,
					"rack":    "R1",
					"chassis": "C1",
					"slot":    "S3",
					"bay":     "B2",
				},
				"parentUID":    "node-parent-" + id,
				"childrenUIDs": []any{"node-c1-" + id, "node-c2-" + id},
				"redfishPath":  "/redfish/v1/Systems/" + id,
				"properties":   map[string]any{"bios": "1.2.3", "cpus": "2"},
			}
		},
		Mutate: func(spec map[string]any) {
			spec["model"] = "XD3000"
			spec["location"].(map[string]any)["rack"] = "R9"
			spec["childrenUIDs"] = []any{"node-only-child"}
			spec["properties"] = map[string]any{"bios": "2.0.0"}
		},
		MergePatch: map[string]any{
			"manufacturer": "Cray",
			"location":     map[string]any{"slot": "S7"},
			"properties":   map[string]any{"memory": "512GiB"},
		},
		CheckMergePatch: func(s *CoreSuite, spec map[string]any) {
			s.Equal("Cray", spec["manufacturer"])
			loc := asMap(s, spec["location"])
			s.Equal("S7", loc["slot"])
			s.Equal("R1", loc["rack"], "merge patch must preserve sibling nested fields")
			props := asMap(s, spec["properties"])
			s.Equal("512GiB", props["memory"])
			s.Equal("1.2.3", props["bios"], "merge patch must preserve existing map keys")
		},
		InvalidSpecs: map[string]func(map[string]any){
			"missing nodeType": func(spec map[string]any) { delete(spec, "nodeType") },
			"empty nodeType":   func(spec map[string]any) { spec["nodeType"] = "" },
			"wrong JSON type":  func(spec map[string]any) { spec["childrenUIDs"] = "not-a-list" },
		},
		NewStatus: func() map[string]any {
			return map[string]any{
				"health":      "OK",
				"state":       "Present",
				"functional":  "Enabled",
				"lastSeen":    "2026-01-02T03:04:05Z",
				"errors":      []any{"fan 2 degraded"},
				"temperature": 41.5,
				"power":       350.25,
				"metrics":     map[string]any{"cpuUtil": 0.75, "memUtil": 0.5},
			}
		},
	}
}

func userKind() resourceKind {
	return resourceKind{
		Kind:      "User",
		Path:      "/users",
		UIDPrefix: "user-",
		NewSpec: func(id string) map[string]any {
			// id is lower-case alphanumeric, so it is a valid username.
			return map[string]any{
				"username": "u" + id,
				"email":    "u" + id + "@example.com",
				"password": "correct-horse-battery",
				"fullName": "Test User " + id,
				"role":     "user",
				"apiKey":   strings.Repeat("k", 40),
				"active":   true,
			}
		},
		Mutate: func(spec map[string]any) {
			spec["fullName"] = "Renamed User"
			spec["role"] = "admin"
			spec["active"] = false
		},
		MergePatch: map[string]any{
			"role":   "readonly",
			"active": false,
		},
		CheckMergePatch: func(s *CoreSuite, spec map[string]any) {
			s.Equal("readonly", spec["role"])
			s.Equal(false, spec["active"])
			s.NotEmpty(spec["email"], "merge patch must preserve untouched fields")
		},
		InvalidSpecs: map[string]func(map[string]any){
			"missing username":   func(spec map[string]any) { delete(spec, "username") },
			"short username":     func(spec map[string]any) { spec["username"] = "ab" },
			"non-alnum username": func(spec map[string]any) { spec["username"] = "bad-name!" },
			"invalid email":      func(spec map[string]any) { spec["email"] = "not-an-email" },
			"short password":     func(spec map[string]any) { spec["password"] = "short" },
			"empty fullName":     func(spec map[string]any) { spec["fullName"] = "" },
			"unknown role":       func(spec map[string]any) { spec["role"] = "superuser" },
			"short apiKey":       func(spec map[string]any) { spec["apiKey"] = "tooshort" },
			"wrong JSON type":    func(spec map[string]any) { spec["active"] = "yes" },
		},
		NewStatus: func() map[string]any {
			return map[string]any{
				"phase":     "Active",
				"message":   "account provisioned",
				"lastLogin": "2026-01-02T03:04:05Z",
			}
		},
	}
}

// --- helpers ---------------------------------------------------------------

var uniqueCounter atomic.Int64

// uniqueID returns a lower-case alphanumeric id unique to this call, so tests
// are independent and can be re-run against the same running service.
func uniqueID() string {
	n := uniqueCounter.Add(1)
	return strconv.FormatInt(time.Now().UnixNano(), 36) + strconv.FormatInt(n, 36)
}

// createBody builds a create/update request body.
func createBody(name string, spec map[string]any, labels map[string]string) map[string]any {
	md := map[string]any{"name": name}
	if len(labels) > 0 {
		md["labels"] = labels
	}
	return map[string]any{"metadata": md, "spec": spec}
}

// create creates a resource of kind k with a fresh spec and returns it along
// with the spec that was sent.
func (s *CoreSuite) create(k resourceKind) (resourceObj, map[string]any) {
	id := uniqueID()
	spec := k.NewSpec(id)
	status, body := s.doJSON(http.MethodPost, k.Path, createBody(strings.ToLower(k.Kind)+"-"+id, spec, nil))
	s.Require().Equal(http.StatusCreated, status, "create %s: %s", k.Kind, body)
	return s.decode(body), spec
}

func (s *CoreSuite) get(k resourceKind, uid string) resourceObj {
	status, body := s.doJSON(http.MethodGet, k.Path+"/"+uid, nil)
	s.Require().Equal(http.StatusOK, status, "GET %s %s: %s", k.Kind, uid, body)
	return s.decode(body)
}

func (s *CoreSuite) list(k resourceKind) []resourceObj {
	status, body := s.doJSON(http.MethodGet, k.Path, nil)
	s.Require().Equal(http.StatusOK, status, "list %s: %s", k.Kind, body)
	var items []resourceObj
	s.Require().NoError(json.Unmarshal(body, &items), "body: %s", body)
	return items
}

func (s *CoreSuite) decode(body []byte) resourceObj {
	var obj resourceObj
	s.Require().NoError(json.Unmarshal(body, &obj), "body: %s", body)
	return obj
}

// doJSON sends payload (marshaled as JSON when non-nil) with Content-Type
// application/json.
func (s *CoreSuite) doJSON(method, path string, payload any) (int, []byte) {
	var data []byte
	if payload != nil {
		var err error
		data, err = json.Marshal(payload)
		s.Require().NoError(err)
	}
	resp, body := s.do(method, path, data, map[string]string{"Content-Type": "application/json"})
	return resp.StatusCode, body
}

// do sends a raw request and returns the response (body already read) and body.
func (s *CoreSuite) do(method, path string, data []byte, headers map[string]string) (*http.Response, []byte) {
	var reqBody io.Reader
	if data != nil {
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, s.baseURL+path, reqBody)
	s.Require().NoError(err)
	for k, v := range headers {
		if data == nil && k == "Content-Type" {
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	s.Require().NoError(err, "%s %s", method, path)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp, body
}

// normalize round-trips v through JSON so it compares equal to decoded
// responses (numbers become float64, slices []any, maps map[string]any).
func normalize(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

func asMap(s *CoreSuite, v any) map[string]any {
	m, ok := v.(map[string]any)
	s.Require().True(ok, "expected a JSON object, got %T (%v)", v, v)
	return m
}

func cloneSpec(spec map[string]any) map[string]any {
	return normalize(spec).(map[string]any)
}

func containsUID(items []resourceObj, uid string) bool {
	for _, it := range items {
		if it.Metadata.UID == uid {
			return true
		}
	}
	return false
}

func (s *CoreSuite) waitForHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := s.client.Get(s.baseURL + "/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			err = fmt.Errorf("status %d", resp.StatusCode)
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

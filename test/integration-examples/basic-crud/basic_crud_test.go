// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package basiccrud tests a service generated from examples/01-basic-crud
// (see resource-defs/device.go) while it runs under fabrun. The tests mirror
// the CRUD flow in that example's test-api.sh.
//
// The service is generated, started, and stopped by the Makefile in this
// directory; `make test-only` runs these tests against an already running
// service. Without FABRICA_EXAMPLE_URL the tests are skipped.
package basiccrud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const devicesPath = "/devices"

type device struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		UID    string            `json:"uid"`
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec map[string]any `json:"spec"`
}

type BasicCRUDSuite struct {
	suite.Suite
	baseURL string
	client  *http.Client
}

func TestBasicCRUD(t *testing.T) {
	suite.Run(t, new(BasicCRUDSuite))
}

func (s *BasicCRUDSuite) SetupSuite() {
	s.baseURL = strings.TrimRight(os.Getenv("FABRICA_EXAMPLE_URL"), "/")
	if s.baseURL == "" {
		s.T().Skip("FABRICA_EXAMPLE_URL is not set; run via 'make test' or 'make up test-only'")
	}
	s.client = &http.Client{Timeout: 10 * time.Second}
	s.Require().NoError(s.waitForHealth(30*time.Second), "service at %s is not healthy; is it running ('make up')?", s.baseURL)
}

// TestCreateGet covers creating a device and reading it back (test-api.sh
// steps 1 and 4): the create echoes the spec and assigns a UID, Kind, and the
// request labels.
func (s *BasicCRUDSuite) TestCreateGet() {
	in := newDeviceInput("switch")
	in.spec["hostname"] = "switch-01.example.com"
	in.spec["ipaddr"] = "192.168.1.100"
	in.spec["description"] = "Core switch"
	in.labels = map[string]string{"environment": "test"}

	status, body := s.post(in.body())
	s.Require().Equal(http.StatusCreated, status, "create: %s", body)

	var created device
	s.Require().NoError(json.Unmarshal(body, &created), "body: %s", body)
	s.NotEmpty(created.Metadata.UID)
	s.Equal("Device", created.Kind)
	s.Equal(in.name, created.Metadata.Name)
	s.Equal("switch-01.example.com", created.Spec["hostname"])
	s.Equal("192.168.1.100", created.Spec["ipaddr"])
	s.Equal("test", created.Metadata.Labels["environment"])

	got := s.get(created.Metadata.UID)
	s.Equal(created.Metadata.UID, got.Metadata.UID)
	s.Equal("switch-01.example.com", got.Spec["hostname"])
}

// TestList covers listing devices (test-api.sh step 3): the created device
// appears in the flat array the API returns.
func (s *BasicCRUDSuite) TestList() {
	in := newDeviceInput("listed")
	status, body := s.post(in.body())
	s.Require().Equal(http.StatusCreated, status, "create: %s", body)
	var created device
	s.Require().NoError(json.Unmarshal(body, &created))

	var found bool
	for _, d := range s.list() {
		if d.Metadata.UID == created.Metadata.UID {
			found = true
			break
		}
	}
	s.True(found, "created device %s not found in list", created.Metadata.UID)
}

// TestUpdate covers updating a device (test-api.sh step 5): a PUT changes the
// mutable spec field and the change is visible on a subsequent GET.
func (s *BasicCRUDSuite) TestUpdate() {
	in := newDeviceInput("upd")
	in.spec["ipaddr"] = "192.168.1.100"
	status, body := s.post(in.body())
	s.Require().Equal(http.StatusCreated, status, "create: %s", body)
	var created device
	s.Require().NoError(json.Unmarshal(body, &created))

	update := in.clone()
	update.spec["ipaddr"] = "192.168.1.101"
	status, body = s.do(http.MethodPut, devicesPath+"/"+created.Metadata.UID, update.body())
	s.Require().Equal(http.StatusOK, status, "update: %s", body)

	got := s.get(created.Metadata.UID)
	s.Equal("192.168.1.101", got.Spec["ipaddr"], "ipaddr was not updated")
}

// TestDelete covers deleting a device (test-api.sh steps 6 and 7): after a
// DELETE the device is gone and a GET returns 404.
func (s *BasicCRUDSuite) TestDelete() {
	in := newDeviceInput("del")
	status, body := s.post(in.body())
	s.Require().Equal(http.StatusCreated, status, "create: %s", body)
	var created device
	s.Require().NoError(json.Unmarshal(body, &created))

	status, body = s.do(http.MethodDelete, devicesPath+"/"+created.Metadata.UID, nil)
	s.Require().Equal(http.StatusOK, status, "delete: %s", body)

	status, _ = s.do(http.MethodGet, devicesPath+"/"+created.Metadata.UID, nil)
	s.Equal(http.StatusNotFound, status, "deleted device should return 404")
}

// TestValidationRejectsBadIP covers validation (test-api.sh step 8): the
// ipaddr field has an `ip` validation tag, so a malformed value is rejected
// with 400 Bad Request.
func (s *BasicCRUDSuite) TestValidationRejectsBadIP() {
	in := newDeviceInput("bad")
	in.spec["ipaddr"] = "not-an-ip"

	status, body := s.post(in.body())
	s.Equal(http.StatusBadRequest, status, "expected 400 for invalid ipaddr: %s", body)
}

// TestOpenAPI covers the OpenAPI endpoint (test-api.sh step 9).
func (s *BasicCRUDSuite) TestOpenAPI() {
	status, body := s.do(http.MethodGet, "/openapi.json", nil)
	s.Require().Equal(http.StatusOK, status, "openapi: %s", body)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(body, &doc), "body: %s", body)
	s.Contains(doc, "openapi")
}

// --- helpers ---------------------------------------------------------------

type deviceInput struct {
	name   string
	spec   map[string]any
	labels map[string]string
}

// newDeviceInput returns a device whose name is unique to this call, so tests
// are independent and can be re-run against the same running service.
func newDeviceInput(prefix string) deviceInput {
	id := prefix + strconv.FormatInt(time.Now().UnixNano(), 36)
	return deviceInput{
		name: id,
		spec: map[string]any{
			"hostname":    id + ".example.com",
			"description": "Test device " + id,
		},
	}
}

func (d deviceInput) clone() deviceInput {
	spec := make(map[string]any, len(d.spec))
	for k, v := range d.spec {
		spec[k] = v
	}
	return deviceInput{name: d.name, spec: spec, labels: d.labels}
}

func (d deviceInput) body() map[string]any {
	metadata := map[string]any{"name": d.name}
	if len(d.labels) > 0 {
		metadata["labels"] = d.labels
	}
	return map[string]any{
		"metadata": metadata,
		"spec":     d.spec,
	}
}

func (s *BasicCRUDSuite) do(method, path string, payload any) (int, []byte) {
	var reqBody io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		s.Require().NoError(err)
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, s.baseURL+path, reqBody)
	s.Require().NoError(err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	s.Require().NoError(err, "%s %s", method, path)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, data
}

func (s *BasicCRUDSuite) post(payload any) (int, []byte) {
	return s.do(http.MethodPost, devicesPath, payload)
}

func (s *BasicCRUDSuite) get(uid string) device {
	status, body := s.do(http.MethodGet, devicesPath+"/"+uid, nil)
	s.Require().Equal(http.StatusOK, status, "GET device %s: %s", uid, body)
	var d device
	s.Require().NoError(json.Unmarshal(body, &d), "body: %s", body)
	return d
}

func (s *BasicCRUDSuite) list() []device {
	status, body := s.do(http.MethodGet, devicesPath, nil)
	s.Require().Equal(http.StatusOK, status, "list devices: %s", body)
	var devices []device
	s.Require().NoError(json.Unmarshal(body, &devices), "body: %s", body)
	return devices
}

func (s *BasicCRUDSuite) waitForHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := s.client.Get(s.baseURL + "/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			err = fmt.Errorf("GET /health returned %d", resp.StatusCode)
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

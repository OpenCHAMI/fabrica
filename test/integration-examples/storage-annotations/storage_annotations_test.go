// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package storageannotations tests a service generated from
// examples/12-storage-annotations (see resource-defs/user.go) while it runs under
// fabrun. The tests mirror the "Testing the API" section of the example README.
//
// The service is generated, started, and stopped by the Makefile in this
// directory; `make test-only` runs these tests against an already running
// service. Without FABRICA_EXAMPLE_URL the tests are skipped.
package storageannotations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const usersPath = "/users"

type user struct {
	Metadata struct {
		UID  string `json:"uid"`
		Name string `json:"name"`
	} `json:"metadata"`
	Spec map[string]any `json:"spec"`
}

type StorageAnnotationsSuite struct {
	suite.Suite
	baseURL string
	client  *http.Client
}

func TestStorageAnnotations(t *testing.T) {
	suite.Run(t, new(StorageAnnotationsSuite))
}

func (s *StorageAnnotationsSuite) SetupSuite() {
	s.baseURL = strings.TrimRight(os.Getenv("FABRICA_EXAMPLE_URL"), "/")
	if s.baseURL == "" {
		s.T().Skip("FABRICA_EXAMPLE_URL is not set; run via 'make test' or 'make up test-only'")
	}
	s.client = &http.Client{Timeout: 10 * time.Second}
	s.Require().NoError(s.waitForHealth(30*time.Second), "service at %s is not healthy; is it running ('make up')?", s.baseURL)
}

// TestCreateUser covers README step 1: the sensitive password is not returned
// and the omitted Active field takes its default.
func (s *StorageAnnotationsSuite) TestCreateUser() {
	u := newUserInput("alice")
	delete(u.spec, "active")

	status, body := s.post(u.body())
	s.Require().Equal(http.StatusCreated, status, "create: %s", body)

	var created user
	s.Require().NoError(json.Unmarshal(body, &created), "body: %s", body)
	s.NotEmpty(created.Metadata.UID)
	s.Equal(u.name, created.Metadata.Name)
	s.Equal(u.spec["username"], created.Spec["username"])
	s.Equal(u.spec["email"], created.Spec["email"])
	s.Equal(u.spec["fullName"], created.Spec["fullName"])
	s.Equal("admin", created.Spec["role"])
	s.Equal(true, created.Spec["active"], "active should default to true")
	s.NotContains(created.Spec, "password", "sensitive field must not be returned")
	s.NotContains(string(body), u.spec["password"], "plaintext password must not be returned")

	got := s.get(created.Metadata.UID)
	s.NotContains(got.Spec, "password", "sensitive field must not be returned by GET")
}

// TestPasswordStoredHashed covers README step 2 without depending on the
// storage schema: the plaintext must not appear anywhere in the database, and
// a bcrypt hash with the annotated cost (12) must.
func (s *StorageAnnotationsSuite) TestPasswordStoredHashed() {
	u := newUserInput("hashed")
	status, body := s.post(u.body())
	s.Require().Equal(http.StatusCreated, status, "create: %s", body)

	dump := s.dumpDatabase()
	s.NotContains(dump, u.spec["password"], "plaintext password found in the database")
	s.Regexp(regexp.MustCompile(`\$2[aby]\$12\$`), dump, "no bcrypt cost-12 hash found in the database")
}

// TestDuplicateUserRejected posts the same user twice.
func (s *StorageAnnotationsSuite) TestDuplicateUserRejected() {
	u := newUserInput("dup")
	status, body := s.post(u.body())
	s.Require().Equal(http.StatusCreated, status, "first create: %s", body)

	status, body = s.post(u.body())
	s.NotEqual(http.StatusCreated, status, "duplicate create succeeded: %s", body)
	s.Len(s.usersWhere("username", u.spec["username"]), 1)
}

// TestDuplicateUsernameRejected covers README step 3.
func (s *StorageAnnotationsSuite) TestDuplicateUsernameRejected() {
	first := newUserInput("uname")
	status, body := s.post(first.body())
	s.Require().Equal(http.StatusCreated, status, "first create: %s", body)

	second := newUserInput("uname2")
	second.spec["username"] = first.spec["username"]
	status, body = s.post(second.body())
	s.NotEqual(http.StatusCreated, status, "create with duplicate username succeeded: %s", body)
	s.Len(s.usersWhere("username", first.spec["username"]), 1)
}

// TestDuplicateEmailRejected covers README step 4.
func (s *StorageAnnotationsSuite) TestDuplicateEmailRejected() {
	first := newUserInput("email")
	status, body := s.post(first.body())
	s.Require().Equal(http.StatusCreated, status, "first create: %s", body)

	second := newUserInput("email2")
	second.spec["email"] = first.spec["email"]
	status, body = s.post(second.body())
	s.NotEqual(http.StatusCreated, status, "create with duplicate email succeeded: %s", body)
	s.Len(s.usersWhere("email", first.spec["email"]), 1)
}

// TestDefaultRole covers README step 5.
func (s *StorageAnnotationsSuite) TestDefaultRole() {
	u := newUserInput("charlie")
	delete(u.spec, "role")

	status, body := s.post(u.body())
	s.Require().Equal(http.StatusCreated, status, "create without role: %s", body)

	var created user
	s.Require().NoError(json.Unmarshal(body, &created), "body: %s", body)
	s.Equal("user", created.Spec["role"], "role should default to user")
}

// TestUsernameImmutable covers README step 6. The update may be rejected or
// accepted with the username left unchanged; either way it must not change.
func (s *StorageAnnotationsSuite) TestUsernameImmutable() {
	u := newUserInput("immut")
	status, body := s.post(u.body())
	s.Require().Equal(http.StatusCreated, status, "create: %s", body)
	var created user
	s.Require().NoError(json.Unmarshal(body, &created), "body: %s", body)

	update := u.clone()
	update.spec["username"] = u.spec["username"].(string) + "new"
	update.spec["fullName"] = "Updated Name"
	status, body = s.do(http.MethodPut, usersPath+"/"+created.Metadata.UID, update.body())

	got := s.get(created.Metadata.UID)
	s.Equal(u.spec["username"], got.Spec["username"], "username changed (PUT returned %d: %s)", status, body)
	if status == http.StatusOK {
		s.Equal("Updated Name", got.Spec["fullName"], "mutable field fullName was not updated")
	}
}

// --- helpers ---------------------------------------------------------------

type userInput struct {
	name string
	spec map[string]any
}

// newUserInput returns a valid user whose name, username, and email are unique
// to this call, so tests are independent and can be re-run against the same
// running service.
func newUserInput(prefix string) userInput {
	id := prefix + strconv.FormatInt(time.Now().UnixNano(), 36)
	return userInput{
		name: id,
		spec: map[string]any{
			"username": id,
			"email":    id + "@example.com",
			"password": "Pw-" + id,
			"fullName": "Test " + id,
			"role":     "admin",
			"active":   true,
		},
	}
}

func (u userInput) clone() userInput {
	spec := make(map[string]any, len(u.spec))
	for k, v := range u.spec {
		spec[k] = v
	}
	return userInput{name: u.name, spec: spec}
}

func (u userInput) body() map[string]any {
	return map[string]any{
		"metadata": map[string]any{"name": u.name},
		"spec":     u.spec,
	}
}

func (s *StorageAnnotationsSuite) do(method, path string, payload any) (int, []byte) {
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

func (s *StorageAnnotationsSuite) post(payload any) (int, []byte) {
	return s.do(http.MethodPost, usersPath, payload)
}

func (s *StorageAnnotationsSuite) get(uid string) user {
	status, body := s.do(http.MethodGet, usersPath+"/"+uid, nil)
	s.Require().Equal(http.StatusOK, status, "GET user %s: %s", uid, body)
	var u user
	s.Require().NoError(json.Unmarshal(body, &u), "body: %s", body)
	return u
}

func (s *StorageAnnotationsSuite) list() []user {
	status, body := s.do(http.MethodGet, usersPath, nil)
	s.Require().Equal(http.StatusOK, status, "list users: %s", body)
	var users []user
	s.Require().NoError(json.Unmarshal(body, &users), "body: %s", body)
	return users
}

// usersWhere returns the listed users whose spec[field] equals value.
func (s *StorageAnnotationsSuite) usersWhere(field string, value any) []user {
	var out []user
	for _, u := range s.list() {
		if u.Spec[field] == value {
			out = append(out, u)
		}
	}
	return out
}

func (s *StorageAnnotationsSuite) waitForHealth(timeout time.Duration) error {
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

// dumpDatabase returns the full contents of the service's database using
// `fabrun sql`. It skips when the fabrun settings are not provided.
func (s *StorageAnnotationsSuite) dumpDatabase() string {
	fabrun := os.Getenv("FABRUN")
	name := os.Getenv("FABRICA_EXAMPLE_NAME")
	if fabrun == "" || name == "" {
		s.T().Skip("FABRUN and FABRICA_EXAMPLE_NAME are required to inspect the database")
	}

	query := ".dump"
	if os.Getenv("FABRICA_EXAMPLE_DB") == "postgres" {
		query = "SELECT table_name, query_to_xml(format('SELECT * FROM %I', table_name), true, false, '') " +
			"FROM information_schema.tables WHERE table_schema = 'public'"
	}
	out, err := exec.Command(fabrun, "sql", "--name", name, query).CombinedOutput()
	s.Require().NoError(err, "fabrun sql: %s", out)
	return string(out)
}

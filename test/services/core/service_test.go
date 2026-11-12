// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package core

import (
	"encoding/json"
	"net/http"
	"strings"
)

// TestHealth covers GET /health.
func (s *CoreSuite) TestHealth() {
	status, body := s.doJSON(http.MethodGet, "/health", nil)
	s.Equal(http.StatusOK, status, "health: %s", body)
}

// TestServiceStatus covers GET /service/status.
func (s *CoreSuite) TestServiceStatus() {
	status, body := s.doJSON(http.MethodGet, "/service/status", nil)
	s.Require().Equal(http.StatusOK, status, "service status: %s", body)
	var got struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	s.Require().NoError(json.Unmarshal(body, &got), "body: %s", body)
	s.Equal("ok", got.Status)
	s.Equal("node-user-service", got.Service)
}

// TestAPICatalog covers GET and HEAD /.well-known/api-catalog (RFC 9727): every
// resource collection is listed as an item.
func (s *CoreSuite) TestAPICatalog() {
	resp, body := s.do(http.MethodGet, "/.well-known/api-catalog", nil, nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "api-catalog: %s", body)
	s.Contains(resp.Header.Get("Content-Type"), "application/linkset+json")

	var catalog struct {
		Linkset []struct {
			Anchor string `json:"anchor"`
			Item   []struct {
				Href string `json:"href"`
			} `json:"item"`
		} `json:"linkset"`
	}
	s.Require().NoError(json.Unmarshal(body, &catalog), "body: %s", body)
	s.Require().NotEmpty(catalog.Linkset)
	var hrefs []string
	for _, it := range catalog.Linkset[0].Item {
		hrefs = append(hrefs, it.Href)
	}
	for _, k := range s.kinds {
		found := false
		for _, h := range hrefs {
			if strings.HasSuffix(h, k.Path) {
				found = true
			}
		}
		s.True(found, "api-catalog items %v do not include %s", hrefs, k.Path)
	}

	resp, body = s.do(http.MethodHead, "/.well-known/api-catalog", nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Empty(body, "HEAD must not return a body")
}

// TestOpenAPI covers GET /openapi.json: every generated REST operation is
// documented.
func (s *CoreSuite) TestOpenAPI() {
	status, body := s.doJSON(http.MethodGet, "/openapi.json", nil)
	s.Require().Equal(http.StatusOK, status, "openapi: %s", body)
	var doc struct {
		OpenAPI string                    `json:"openapi"`
		Paths   map[string]map[string]any `json:"paths"`
	}
	s.Require().NoError(json.Unmarshal(body, &doc), "body: %s", body)
	s.NotEmpty(doc.OpenAPI)

	for _, k := range s.kinds {
		s.Run(k.Kind, func() {
			ops := map[string][]string{
				k.Path:                   {"get", "post"},
				k.Path + "/{uid}":        {"get", "put", "patch", "delete"},
				k.Path + "/{uid}/status": {"put", "patch"},
			}
			for path, methods := range ops {
				item, ok := doc.Paths[path]
				if !s.True(ok, "openapi has no path %s", path) {
					continue
				}
				for _, m := range methods {
					s.Contains(item, m, "openapi path %s has no %s operation", path, m)
				}
			}
		})
	}
}

// TestDocs covers GET /docs (Swagger UI).
func (s *CoreSuite) TestDocs() {
	resp, body := s.do(http.MethodGet, "/docs", nil, nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "docs: %s", body)
	s.Contains(resp.Header.Get("Content-Type"), "text/html")
}

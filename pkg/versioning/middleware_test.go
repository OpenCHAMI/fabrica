// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package versioning

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type versionResult struct {
	requested string
	served    string
	group     string
	kind      string
	body      string
	status    int
}

func runVersionRequest(t *testing.T, registry *VersionRegistry, method, path, payload string, headers map[string]string) versionResult {
	t.Helper()

	result := versionResult{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := GetVersionContext(r.Context())
		result.requested = ctx.RequestedVersion
		result.served = ctx.ServeVersion
		result.group = ctx.GroupVersion
		result.kind = ctx.ResourceKind

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		result.body = string(body)
		w.WriteHeader(http.StatusOK)
	})

	middleware := VersionNegotiationMiddleware(registry, nil)

	req := httptest.NewRequest(method, path, bytes.NewBufferString(payload))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)
	result.status = rec.Code

	return result
}

func registerDeviceVersions(t *testing.T, registry *VersionRegistry, defaultVersion string, versions ...string) {
	t.Helper()

	for _, version := range versions {
		info := ResourceTypeInfo{Metadata: SchemaVersion{Version: version}}
		if version == defaultVersion {
			info.Metadata.IsDefault = true
		}
		if err := registry.RegisterVersion("Device", version, info); err != nil {
			t.Fatalf("register %s: %v", version, err)
		}
	}
}

func TestVersionNegotiationUsesBodyAPIVersion(t *testing.T) {
	registry := NewVersionRegistry()
	registerDeviceVersions(t, registry, "v1", "v1", "v2")

	payload := `{"apiVersion":"infra.example.io/v2","kind":"Device","metadata":{"name":"device-1"},"spec":{"ipAddress":"192.168.1.100"}}`
	result := runVersionRequest(t, registry, http.MethodPost, "/devices", payload, map[string]string{
		"Accept": "application/json;version=v1",
	})

	if result.status != http.StatusOK {
		t.Fatalf("unexpected status: %d", result.status)
	}
	if result.requested != "v2" {
		t.Fatalf("expected requested version v2, got %q", result.requested)
	}
	if result.served != "v2" {
		t.Fatalf("expected served version v2, got %q", result.served)
	}
	if result.body == "" || !bytes.Contains([]byte(result.body), []byte("apiVersion")) {
		t.Fatalf("expected body to be preserved, got %q", result.body)
	}
}

func TestVersionNegotiationUsesAcceptWhenBodyMissingVersion(t *testing.T) {
	registry := NewVersionRegistry()
	registerDeviceVersions(t, registry, "v1", "v1", "v2")

	payload := `{"kind":"Device","metadata":{"name":"device-1"},"spec":{"ipAddress":"192.168.1.100"}}`
	result := runVersionRequest(t, registry, http.MethodPost, "/devices", payload, map[string]string{
		"Accept": "application/json;version=v2",
	})

	if result.status != http.StatusOK {
		t.Fatalf("unexpected status: %d", result.status)
	}
	if result.requested != "v2" {
		t.Fatalf("expected requested version v2, got %q", result.requested)
	}
	if result.served != "v2" {
		t.Fatalf("expected served version v2, got %q", result.served)
	}
}

func TestVersionNegotiationRejectsUnsupportedVersion(t *testing.T) {
	registry := NewVersionRegistry()
	registerDeviceVersions(t, registry, "v1", "v1")

	result := runVersionRequest(t, registry, http.MethodGet, "/devices", "", map[string]string{
		"Accept": "application/json;version=v9",
	})

	if result.status != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d", result.status)
	}
}

func TestVersionNegotiationIgnoresMalformedBody(t *testing.T) {
	registry := NewVersionRegistry()
	registerDeviceVersions(t, registry, "v1", "v1")

	payload := `{"apiVersion":"v2"`
	result := runVersionRequest(t, registry, http.MethodPost, "/devices", payload, map[string]string{
		"Accept": "application/json;version=v1",
	})

	if result.status != http.StatusOK {
		t.Fatalf("unexpected status: %d", result.status)
	}
	if result.requested != "v1" {
		t.Fatalf("expected requested version v1, got %q", result.requested)
	}
	if result.served != "v1" {
		t.Fatalf("expected served version v1, got %q", result.served)
	}
}

func TestVersionNegotiationParsesGroupAPIVersion(t *testing.T) {
	registry := NewVersionRegistry()
	registerDeviceVersions(t, registry, "v1", "v1", "v2beta1")

	payload := `{"apiVersion":"infra.example.io/v2beta1","kind":"Device","metadata":{"name":"device-1"},"spec":{}}`
	result := runVersionRequest(t, registry, http.MethodPost, "/devices", payload, nil)

	if result.status != http.StatusOK {
		t.Fatalf("unexpected status: %d", result.status)
	}
	if result.requested != "v2beta1" {
		t.Fatalf("expected requested version v2beta1, got %q", result.requested)
	}
	if result.served != "v2beta1" {
		t.Fatalf("expected served version v2beta1, got %q", result.served)
	}
}

func TestVersionNegotiationUsesURLVersionWhenExplicit(t *testing.T) {
	registry := NewVersionRegistry()
	registerDeviceVersions(t, registry, "v1", "v1", "v2")

	result := runVersionRequest(t, registry, http.MethodGet, "/apis/example/v2/devices", "", nil)

	if result.status != http.StatusOK {
		t.Fatalf("unexpected status: %d", result.status)
	}
	if result.requested != "v2" {
		t.Fatalf("expected requested version v2, got %q", result.requested)
	}
	if result.served != "v2" {
		t.Fatalf("expected served version v2, got %q", result.served)
	}
}

func TestVersionNegotiationDefaultsToGroupVersionWithNoRegisteredVersions(t *testing.T) {
	registry := NewVersionRegistry()

	result := runVersionRequest(t, registry, http.MethodGet, "/devices", "", nil)

	if result.status != http.StatusOK {
		t.Fatalf("unexpected status: %d", result.status)
	}
	if result.served != "v1" {
		t.Fatalf("expected served version v1, got %q", result.served)
	}
}

func TestVersionNegotiationRejectsRequestedVersionWithNoRegisteredVersions(t *testing.T) {
	registry := NewVersionRegistry()

	result := runVersionRequest(t, registry, http.MethodGet, "/devices", "", map[string]string{
		"Accept": "application/json;version=v2",
	})

	if result.status != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d", result.status)
	}
}

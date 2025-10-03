// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package conditional

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultETagGenerator(t *testing.T) {
	data := []byte(`{"name":"test","value":123}`)
	etag := DefaultETagGenerator(data)

	if etag == "" {
		t.Error("ETag should not be empty")
	}

	if etag[0] != '"' || etag[len(etag)-1] != '"' {
		t.Error("ETag should be quoted")
	}

	// Same data should generate same ETag
	etag2 := DefaultETagGenerator(data)
	if etag != etag2 {
		t.Error("Same data should generate same ETag")
	}

	// Different data should generate different ETag
	data2 := []byte(`{"name":"test","value":456}`)
	etag3 := DefaultETagGenerator(data2)
	if etag == etag3 {
		t.Error("Different data should generate different ETag")
	}
}

func TestWeakETagGenerator(t *testing.T) {
	data := []byte(`{"name":"test"}`)
	etag := WeakETagGenerator(data)

	if !hasPrefix(etag, "W/") {
		t.Error("Weak ETag should start with W/")
	}
}

func TestParseETag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"abc123"`, "abc123"},
		{`W/"abc123"`, "abc123"},
		{`abc123`, "abc123"},
	}

	for _, test := range tests {
		result := ParseETag(test.input)
		if result != test.expected {
			t.Errorf("ParseETag(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestMatchesETag(t *testing.T) {
	etag := `"abc123"`

	tests := []struct {
		ifMatch  string
		expected bool
	}{
		{`"abc123"`, true},
		{`"xyz789"`, false},
		{`*`, true},
		{`"abc123", "xyz789"`, true},
		{`"xyz789", "def456"`, false},
	}

	for _, test := range tests {
		result := MatchesETag(test.ifMatch, etag)
		if result != test.expected {
			t.Errorf("MatchesETag(%q, %q) = %v, want %v", test.ifMatch, etag, result, test.expected)
		}
	}
}

func TestParseHTTPDate(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"Mon, 02 Jan 2006 15:04:05 MST", false},  // RFC1123
		{"Monday, 02-Jan-06 15:04:05 MST", false}, // RFC850
		{"Mon Jan  2 15:04:05 2006", false},       // ANSIC
		{"invalid date", true},
	}

	for _, test := range tests {
		_, err := ParseHTTPDate(test.input)
		if (err != nil) != test.wantErr {
			t.Errorf("ParseHTTPDate(%q) error = %v, wantErr %v", test.input, err, test.wantErr)
		}
	}
}

func TestCheckConditionalRequest_IfMatch(t *testing.T) {
	etag := `"abc123"`
	lastModified := time.Now()

	// Matching ETag should pass
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/resource", nil)
	r.Header.Set("If-Match", etag)

	handled := CheckConditionalRequest(w, r, etag, lastModified)
	if handled {
		t.Error("Matching If-Match should not be handled")
	}

	// Non-matching ETag should fail with 412
	w = httptest.NewRecorder()
	r = httptest.NewRequest("PUT", "/resource", nil)
	r.Header.Set("If-Match", `"xyz789"`)

	handled = CheckConditionalRequest(w, r, etag, lastModified)
	if !handled {
		t.Error("Non-matching If-Match should be handled")
	}
	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("Expected status %d, got %d", http.StatusPreconditionFailed, w.Code)
	}
}

func TestCheckConditionalRequest_IfNoneMatch(t *testing.T) {
	etag := `"abc123"`
	lastModified := time.Now()

	// GET with matching If-None-Match should return 304
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/resource", nil)
	r.Header.Set("If-None-Match", etag)

	handled := CheckConditionalRequest(w, r, etag, lastModified)
	if !handled {
		t.Error("Matching If-None-Match on GET should be handled")
	}
	if w.Code != http.StatusNotModified {
		t.Errorf("Expected status %d, got %d", http.StatusNotModified, w.Code)
	}

	// PUT with matching If-None-Match should return 412
	w = httptest.NewRecorder()
	r = httptest.NewRequest("PUT", "/resource", nil)
	r.Header.Set("If-None-Match", etag)

	handled = CheckConditionalRequest(w, r, etag, lastModified)
	if !handled {
		t.Error("Matching If-None-Match on PUT should be handled")
	}
	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("Expected status %d, got %d", http.StatusPreconditionFailed, w.Code)
	}
}

func TestCheckConditionalRequest_IfUnmodifiedSince(t *testing.T) {
	etag := `"abc123"`
	lastModified := time.Now()
	beforeModified := lastModified.Add(-1 * time.Hour)

	// Request with If-Unmodified-Since before modification should fail
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/resource", nil)
	r.Header.Set("If-Unmodified-Since", beforeModified.Format(time.RFC1123))

	handled := CheckConditionalRequest(w, r, etag, lastModified)
	if !handled {
		t.Error("If-Unmodified-Since before modification should be handled")
	}
	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("Expected status %d, got %d", http.StatusPreconditionFailed, w.Code)
	}

	// Request with If-Unmodified-Since after modification should pass
	afterModified := lastModified.Add(1 * time.Hour)
	w = httptest.NewRecorder()
	r = httptest.NewRequest("PUT", "/resource", nil)
	r.Header.Set("If-Unmodified-Since", afterModified.Format(time.RFC1123))

	handled = CheckConditionalRequest(w, r, etag, lastModified)
	if handled {
		t.Error("If-Unmodified-Since after modification should not be handled")
	}
}

func TestCheckConditionalRequest_IfModifiedSince(t *testing.T) {
	etag := `"abc123"`
	lastModified := time.Now().Truncate(time.Second) // Truncate to second precision
	beforeModified := lastModified.Add(-1 * time.Hour)
	afterModified := lastModified.Add(1 * time.Hour)

	// GET with If-Modified-Since at exact modification time should return 304
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/resource", nil)
	r.Header.Set("If-Modified-Since", lastModified.Format(time.RFC1123))

	handled := CheckConditionalRequest(w, r, etag, lastModified)
	if !handled {
		t.Error("If-Modified-Since at modification time should be handled")
	}
	if w.Code != http.StatusNotModified {
		t.Errorf("Expected status %d, got %d", http.StatusNotModified, w.Code)
	}

	// GET with If-Modified-Since after modification should return 304
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/resource", nil)
	r.Header.Set("If-Modified-Since", afterModified.Format(time.RFC1123))

	handled = CheckConditionalRequest(w, r, etag, lastModified)
	if !handled {
		t.Error("If-Modified-Since after modification should be handled")
	}
	if w.Code != http.StatusNotModified {
		t.Errorf("Expected status %d, got %d", http.StatusNotModified, w.Code)
	}

	// GET with If-Modified-Since before modification should pass
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/resource", nil)
	r.Header.Set("If-Modified-Since", beforeModified.Format(time.RFC1123))

	handled = CheckConditionalRequest(w, r, etag, lastModified)
	if handled {
		t.Error("If-Modified-Since before modification should not be handled")
	}
}

func TestSetETag(t *testing.T) {
	w := httptest.NewRecorder()
	etag := `"abc123"`
	SetETag(w, etag)

	if w.Header().Get("ETag") != etag {
		t.Errorf("Expected ETag %q, got %q", etag, w.Header().Get("ETag"))
	}
}

func TestSetLastModified(t *testing.T) {
	w := httptest.NewRecorder()
	now := time.Now()
	SetLastModified(w, now)

	header := w.Header().Get("Last-Modified")
	if header == "" {
		t.Error("Last-Modified header should be set")
	}

	// Parse and verify
	parsed, err := ParseHTTPDate(header)
	if err != nil {
		t.Errorf("Failed to parse Last-Modified header: %v", err)
	}

	// Should be within 1 second (accounting for formatting precision)
	if parsed.Unix() != now.Unix() {
		t.Errorf("Last-Modified time mismatch: got %v, want %v", parsed.Unix(), now.Unix())
	}
}

func TestSetCacheControl(t *testing.T) {
	tests := []struct {
		name     string
		opts     CacheControlOptions
		expected string
	}{
		{
			name:     "no-store",
			opts:     CacheControlOptions{NoStore: true},
			expected: "no-store",
		},
		{
			name:     "private with max-age",
			opts:     CacheControlOptions{Private: true, MaxAge: 3600},
			expected: "private, max-age=3600",
		},
		{
			name:     "public with must-revalidate",
			opts:     CacheControlOptions{Public: true, MustRevalidate: true},
			expected: "public, must-revalidate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SetCacheControl(w, test.opts)

			header := w.Header().Get("Cache-Control")
			if header != test.expected {
				t.Errorf("Expected Cache-Control %q, got %q", test.expected, header)
			}
		})
	}
}

func TestVaryHeader(t *testing.T) {
	w := httptest.NewRecorder()
	VaryHeader(w, "Accept", "Accept-Encoding")

	header := w.Header().Get("Vary")
	expected := "Accept, Accept-Encoding"
	if header != expected {
		t.Errorf("Expected Vary %q, got %q", expected, header)
	}

	// Adding more should append
	VaryHeader(w, "Authorization")
	header = w.Header().Get("Vary")
	if !contains(header, "Authorization") {
		t.Errorf("Expected Authorization in Vary header, got %q", header)
	}
}

func TestGenerateResourceETag(t *testing.T) {
	data := []byte(`{"name":"test"}`)
	version := "v1"
	modTime := time.Now()

	etag1 := GenerateResourceETag(data, version, modTime)
	if etag1 == "" {
		t.Error("ETag should not be empty")
	}

	// Same inputs should generate same ETag
	etag2 := GenerateResourceETag(data, version, modTime)
	if etag1 != etag2 {
		t.Error("Same inputs should generate same ETag")
	}

	// Different version should generate different ETag
	etag3 := GenerateResourceETag(data, "v2", modTime)
	if etag1 == etag3 {
		t.Error("Different version should generate different ETag")
	}
}

// Helper functions
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || hasPrefix(s, substr) ||
		hasPrefix(s[len(s)-len(substr):], substr) ||
		containsMiddle(s, substr))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package conditional provides middleware and utilities for conditional HTTP requests.
//
// This package implements support for conditional requests as defined in RFC 7232,
// including If-Match, If-None-Match, If-Modified-Since, and If-Unmodified-Since headers.
//
// Usage:
//
//	// Add ETags to responses
//	handler := conditional.ETagMiddleware(myHandler)
//
//	// Check conditional request headers
//	if conditional.CheckConditionalRequest(w, r, etag, lastModified) {
//	    return // Response already sent (304 or 412)
//	}
package conditional

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ETagGenerator is a function that generates an ETag for a resource
type ETagGenerator func(data []byte) string

// DefaultETagGenerator generates ETags using SHA-256 hash
func DefaultETagGenerator(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf(`"%s"`, hex.EncodeToString(hash[:16])) // Use first 16 bytes for brevity
}

// WeakETagGenerator generates weak ETags
func WeakETagGenerator(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf(`W/"%s"`, hex.EncodeToString(hash[:16]))
}

// ParseETag removes weak prefix and quotes from ETag
func ParseETag(etag string) string {
	etag = strings.TrimPrefix(etag, "W/")
	etag = strings.Trim(etag, `"`)
	return etag
}

// MatchesETag checks if the provided ETag matches the expected ETag
func MatchesETag(ifMatch string, etag string) bool {
	if ifMatch == "*" {
		return true
	}

	// Parse multiple ETags (comma-separated)
	tags := strings.Split(ifMatch, ",")
	expectedETag := ParseETag(etag)

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if ParseETag(tag) == expectedETag {
			return true
		}
	}

	return false
}

// ParseHTTPDate parses HTTP date formats
func ParseHTTPDate(dateStr string) (time.Time, error) {
	// Try RFC1123 format first (preferred)
	t, err := time.Parse(time.RFC1123, dateStr)
	if err == nil {
		return t, nil
	}

	// Try RFC850 format
	t, err = time.Parse(time.RFC850, dateStr)
	if err == nil {
		return t, nil
	}

	// Try ANSIC format
	return time.Parse(time.ANSIC, dateStr)
}

// CheckConditionalRequest validates conditional request headers and sends appropriate responses.
// Returns true if a response was sent (304 Not Modified or 412 Precondition Failed).
func CheckConditionalRequest(w http.ResponseWriter, r *http.Request, etag string, lastModified time.Time) bool {
	// Handle If-Match (typically used with PUT, PATCH, DELETE)
	if ifMatch := r.Header.Get("If-Match"); ifMatch != "" {
		if !MatchesETag(ifMatch, etag) {
			w.WriteHeader(http.StatusPreconditionFailed)
			return true
		}
	}

	// Handle If-None-Match (typically used with GET, HEAD)
	if ifNoneMatch := r.Header.Get("If-None-Match"); ifNoneMatch != "" {
		if MatchesETag(ifNoneMatch, etag) {
			// For GET/HEAD, return 304 Not Modified
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				w.WriteHeader(http.StatusNotModified)
				return true
			}
			// For other methods, return 412 Precondition Failed
			w.WriteHeader(http.StatusPreconditionFailed)
			return true
		}
	}

	// Handle If-Unmodified-Since (typically used with PUT, PATCH, DELETE)
	if ifUnmodifiedSince := r.Header.Get("If-Unmodified-Since"); ifUnmodifiedSince != "" {
		if t, err := ParseHTTPDate(ifUnmodifiedSince); err == nil {
			if lastModified.After(t) {
				w.WriteHeader(http.StatusPreconditionFailed)
				return true
			}
		}
	}

	// Handle If-Modified-Since (typically used with GET, HEAD)
	if ifModifiedSince := r.Header.Get("If-Modified-Since"); ifModifiedSince != "" {
		if t, err := ParseHTTPDate(ifModifiedSince); err == nil {
			if !lastModified.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return true
			}
		}
	}

	return false
}

// SetETag sets the ETag header on the response
func SetETag(w http.ResponseWriter, etag string) {
	w.Header().Set("ETag", etag)
}

// SetLastModified sets the Last-Modified header on the response
func SetLastModified(w http.ResponseWriter, t time.Time) {
	w.Header().Set("Last-Modified", t.UTC().Format(time.RFC1123))
}

// ETagMiddleware automatically adds ETags to responses
func ETagMiddleware(generator ETagGenerator) func(http.Handler) http.Handler {
	if generator == nil {
		generator = DefaultETagGenerator
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use a response writer that captures the response
			crw := &conditionalResponseWriter{
				ResponseWriter: w,
				generator:      generator,
			}

			next.ServeHTTP(crw, r)
		})
	}
}

// conditionalResponseWriter wraps http.ResponseWriter to add ETags
type conditionalResponseWriter struct {
	http.ResponseWriter
	generator ETagGenerator
	written   bool
}

func (crw *conditionalResponseWriter) Write(data []byte) (int, error) {
	if !crw.written {
		// Generate and set ETag before first write
		if crw.generator != nil {
			etag := crw.generator(data)
			SetETag(crw.ResponseWriter, etag)
		}
		crw.written = true
	}
	return crw.ResponseWriter.Write(data)
}

// ConditionalRequestInfo contains information extracted from conditional request headers
type ConditionalRequestInfo struct {
	IfMatch           string
	IfNoneMatch       string
	IfModifiedSince   time.Time
	IfUnmodifiedSince time.Time
}

// ExtractConditionalInfo extracts conditional request information from headers
func ExtractConditionalInfo(r *http.Request) *ConditionalRequestInfo {
	info := &ConditionalRequestInfo{
		IfMatch:     r.Header.Get("If-Match"),
		IfNoneMatch: r.Header.Get("If-None-Match"),
	}

	if ifModifiedSince := r.Header.Get("If-Modified-Since"); ifModifiedSince != "" {
		if t, err := ParseHTTPDate(ifModifiedSince); err == nil {
			info.IfModifiedSince = t
		}
	}

	if ifUnmodifiedSince := r.Header.Get("If-Unmodified-Since"); ifUnmodifiedSince != "" {
		if t, err := ParseHTTPDate(ifUnmodifiedSince); err == nil {
			info.IfUnmodifiedSince = t
		}
	}

	return info
}

// ValidateConditional validates conditional headers against current resource state
func ValidateConditional(info *ConditionalRequestInfo, currentETag string, currentModified time.Time) (valid bool, statusCode int) {
	// Check If-Match
	if info.IfMatch != "" {
		if !MatchesETag(info.IfMatch, currentETag) {
			return false, http.StatusPreconditionFailed
		}
	}

	// Check If-None-Match
	if info.IfNoneMatch != "" {
		if MatchesETag(info.IfNoneMatch, currentETag) {
			return false, http.StatusPreconditionFailed
		}
	}

	// Check If-Unmodified-Since
	if !info.IfUnmodifiedSince.IsZero() {
		if currentModified.After(info.IfUnmodifiedSince) {
			return false, http.StatusPreconditionFailed
		}
	}

	// Check If-Modified-Since
	if !info.IfModifiedSince.IsZero() {
		if !currentModified.After(info.IfModifiedSince) {
			return false, http.StatusNotModified
		}
	}

	return true, http.StatusOK
}

// GenerateResourceETag generates an ETag for a resource based on its content and metadata
func GenerateResourceETag(resourceData []byte, resourceVersion string, modifiedTime time.Time) string {
	// Combine resource data, version, and modification time for ETag
	combined := fmt.Sprintf("%s|%s|%d", resourceData, resourceVersion, modifiedTime.Unix())
	return DefaultETagGenerator([]byte(combined))
}

// CacheControlOptions defines caching behavior
type CacheControlOptions struct {
	MaxAge          int  // Maximum age in seconds
	NoCache         bool // Don't use cache without revalidation
	NoStore         bool // Don't store in cache
	Private         bool // Cache is private (user-specific)
	Public          bool // Cache is public (shared)
	MustRevalidate  bool // Must revalidate when stale
	ProxyRevalidate bool // Proxies must revalidate
	Immutable       bool // Resource never changes
	SMaxAge         int  // Shared cache maximum age
}

// SetCacheControl sets Cache-Control header based on options
func SetCacheControl(w http.ResponseWriter, opts CacheControlOptions) {
	var directives []string

	if opts.NoStore {
		directives = append(directives, "no-store")
	}

	if opts.NoCache {
		directives = append(directives, "no-cache")
	}

	if opts.Private {
		directives = append(directives, "private")
	} else if opts.Public {
		directives = append(directives, "public")
	}

	if opts.MaxAge > 0 {
		directives = append(directives, fmt.Sprintf("max-age=%d", opts.MaxAge))
	}

	if opts.SMaxAge > 0 {
		directives = append(directives, fmt.Sprintf("s-maxage=%d", opts.SMaxAge))
	}

	if opts.MustRevalidate {
		directives = append(directives, "must-revalidate")
	}

	if opts.ProxyRevalidate {
		directives = append(directives, "proxy-revalidate")
	}

	if opts.Immutable {
		directives = append(directives, "immutable")
	}

	if len(directives) > 0 {
		w.Header().Set("Cache-Control", strings.Join(directives, ", "))
	}
}

// VaryHeader adds a Vary header to indicate which request headers affect the response
func VaryHeader(w http.ResponseWriter, headers ...string) {
	existing := w.Header().Get("Vary")
	if existing != "" {
		headers = append([]string{existing}, headers...)
	}
	w.Header().Set("Vary", strings.Join(headers, ", "))
}

// GetResourceVersion extracts resource version from metadata or computes it
func GetResourceVersion(modifiedTime time.Time) string {
	return strconv.FormatInt(modifiedTime.Unix(), 10)
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PatchHandler wraps a handler to provide PATCH support
type PatchHandler struct {
	GetResource   func(r *http.Request) ([]byte, error)
	SaveResource  func(r *http.Request, data []byte) error
	Options       PatchOptions
	ETagGenerator func(data []byte) string
}

// ServeHTTP handles PATCH requests
func (ph *PatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle PATCH requests
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current resource state
	original, err := ph.GetResource(r)
	if err != nil {
		respondError(w, http.StatusNotFound, fmt.Errorf("resource not found: %w", err))
		return
	}

	// Check ETag if required
	if ph.Options.RequireETag {
		etag := r.Header.Get("If-Match")
		if etag == "" {
			respondError(w, http.StatusPreconditionRequired, fmt.Errorf("If-Match header required"))
			return
		}

		// Validate ETag
		currentETag := ph.ETagGenerator(original)
		if etag != currentETag {
			respondError(w, http.StatusPreconditionFailed, fmt.Errorf("ETag mismatch"))
			return
		}
	}

	// Read patch document
	patchData, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Errorf("failed to read request body: %w", err))
		return
	}
	defer r.Body.Close()

	// Detect patch type from Content-Type header
	patchType := DetectPatchType(r.Header.Get("Content-Type"))

	// Validate patch based on type
	if patchType == JSONPatch {
		if err := ValidateJSONPatch(patchData); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
	}

	// Apply patch with options
	result, err := ApplyPatchWithOptions(original, patchData, patchType, ph.Options)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, fmt.Errorf("failed to apply patch: %w", err))
		return
	}

	// If dry-run, return what would be changed
	if ph.Options.DryRun {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
		return
	}

	// Save updated resource
	if result.Modified {
		if err := ph.SaveResource(r, result.Updated); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to save resource: %w", err))
			return
		}
	}

	// Set new ETag
	if ph.ETagGenerator != nil {
		newETag := ph.ETagGenerator(result.Updated)
		w.Header().Set("ETag", newETag)
	}

	// Return updated resource
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(result.Updated)
}

// PatchMiddleware provides automatic PATCH support for PUT/POST handlers
func PatchMiddleware(getFunc func(*http.Request) ([]byte, error), saveFunc func(*http.Request, []byte) error) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only intercept PATCH requests
			if r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			// Handle PATCH
			handler := &PatchHandler{
				GetResource:  getFunc,
				SaveResource: saveFunc,
				Options: PatchOptions{
					AllowAddFields:    true,
					AllowRemoveFields: true,
				},
			}
			handler.ServeHTTP(w, r)
		})
	}
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}

// AutoPatchMiddleware automatically generates PATCH from existing GET and PUT handlers
// This middleware intercepts PATCH requests and translates them to GET+modify+PUT
func AutoPatchMiddleware(basePath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only handle PATCH requests
			if r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			// Read patch document
			patchData, err := io.ReadAll(r.Body)
			if err != nil {
				respondError(w, http.StatusBadRequest, fmt.Errorf("failed to read patch: %w", err))
				return
			}
			r.Body.Close()

			// Create a GET request to fetch current state
			getReq := &http.Request{
				Method: http.MethodGet,
				URL:    r.URL,
				Header: r.Header,
			}

			// Capture GET response
			getRecorder := &responseRecorder{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
			}

			// Execute GET
			next.ServeHTTP(getRecorder, getReq)

			if getRecorder.statusCode != http.StatusOK {
				// If GET failed, return that error
				w.WriteHeader(getRecorder.statusCode)
				w.Write(getRecorder.body.Bytes())
				return
			}

			// Get original resource
			original := getRecorder.body.Bytes()

			// Detect patch type and apply
			patchType := DetectPatchType(r.Header.Get("Content-Type"))
			updated, err := ApplyPatch(original, patchData, patchType)
			if err != nil {
				respondError(w, http.StatusUnprocessableEntity, err)
				return
			}

			// Create PUT request with updated resource
			putReq := &http.Request{
				Method: http.MethodPut,
				URL:    r.URL,
				Header: r.Header,
				Body:   io.NopCloser(bytes.NewReader(updated)),
			}

			// Execute PUT
			next.ServeHTTP(w, putReq)
		})
	}
}

// responseRecorder captures response data
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(data []byte) (int, error) {
	return rr.body.Write(data)
}

// PatchSupport adds Content-Type header advertisement for PATCH support
func PatchSupport(w http.ResponseWriter) {
	accept := w.Header().Get("Accept-Patch")
	patches := []string{
		string(JSONMergePatch),
		string(JSONPatch),
		string(ShorthandPatch),
	}

	if accept == "" {
		w.Header().Set("Accept-Patch", patches[0])
		for _, p := range patches[1:] {
			w.Header().Add("Accept-Patch", p)
		}
	}
}

// OptionsPatchHandler handles OPTIONS requests to advertise PATCH support
func OptionsPatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Allow", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		PatchSupport(w)
		w.WriteHeader(http.StatusOK)
		return
	}
}

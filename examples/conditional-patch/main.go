// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package main demonstrates conditional requests and PATCH operations
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/alexlovelltroy/fabrica/pkg/conditional"
	"github.com/alexlovelltroy/fabrica/pkg/patch"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Resource represents a simple resource
type Resource struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Status      string                 `json:"status"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	ModifiedAt  time.Time              `json:"modifiedAt"`
}

// Simple in-memory storage
var resources = map[string]*Resource{
	"1": {
		ID:          "1",
		Name:        "Example Resource",
		Description: "This is an example",
		Status:      "active",
		Tags:        []string{"example", "demo"},
		Metadata: map[string]interface{}{
			"owner": "admin",
		},
		CreatedAt:  time.Now().Add(-24 * time.Hour),
		ModifiedAt: time.Now().Add(-1 * time.Hour),
	},
}

func main() {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Add ETag middleware
	r.Use(conditional.ETagMiddleware(nil))

	// Routes
	r.Get("/resources", listResources)
	r.Get("/resources/{id}", getResource)
	r.Post("/resources", createResource)
	r.Put("/resources/{id}", updateResource)
	r.Patch("/resources/{id}", patchResource)
	r.Delete("/resources/{id}", deleteResource)
	r.Options("/resources/{id}", optionsResource)

	fmt.Println("Server starting on :8080")
	fmt.Println("\nExample requests:")
	fmt.Println("  # Get resource with ETag")
	fmt.Println("  curl -i http://localhost:8080/resources/1")
	fmt.Println("\n  # Conditional GET (will return 304 if not modified)")
	fmt.Println(`  curl -i -H "If-None-Match: \"etag-value\"" http://localhost:8080/resources/1`)
	fmt.Println("\n  # JSON Merge Patch")
	fmt.Println(`  curl -X PATCH http://localhost:8080/resources/1 \`)
	fmt.Println(`    -H "Content-Type: application/merge-patch+json" \`)
	fmt.Println(`    -d '{"status":"inactive","description":"Updated"}' | jq`)
	fmt.Println("\n  # JSON Patch")
	fmt.Println(`  curl -X PATCH http://localhost:8080/resources/1 \`)
	fmt.Println(`    -H "Content-Type: application/json-patch+json" \`)
	fmt.Println(`    -d '[{"op":"replace","path":"/status","value":"pending"}]' | jq`)
	fmt.Println("\n  # Shorthand Patch")
	fmt.Println(`  curl -X PATCH http://localhost:8080/resources/1 \`)
	fmt.Println(`    -H "Content-Type: application/shorthand-patch+json" \`)
	fmt.Println(`    -d '{"name":"Updated Name","metadata.version":"2.0"}' | jq`)
	fmt.Println("\n  # Update with optimistic concurrency (get ETag first, then use If-Match)")
	fmt.Println(`  curl -X PATCH http://localhost:8080/resources/1 \`)
	fmt.Println(`    -H "If-Match: \"etag-from-previous-get\"" \`)
	fmt.Println(`    -H "Content-Type: application/merge-patch+json" \`)
	fmt.Println(`    -d '{"status":"active"}' | jq`)

	log.Fatal(http.ListenAndServe(":8080", r))
}

func listResources(w http.ResponseWriter, r *http.Request) {
	// Set cache control
	conditional.SetCacheControl(w, conditional.CacheControlOptions{
		Public: true,
		MaxAge: 60, // Cache for 1 minute
	})

	resourceList := make([]*Resource, 0, len(resources))
	for _, res := range resources {
		resourceList = append(resourceList, res)
	}

	respondJSON(w, http.StatusOK, resourceList)
}

func getResource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	resource, exists := resources[id]
	if !exists {
		respondError(w, http.StatusNotFound, fmt.Errorf("resource not found"))
		return
	}

	// Marshal to get ETag
	resourceJSON, _ := json.Marshal(resource)
	etag := conditional.DefaultETagGenerator(resourceJSON)

	// Check conditional request headers
	if conditional.CheckConditionalRequest(w, r, etag, resource.ModifiedAt) {
		return // Response already sent (304 or 412)
	}

	// Set response headers
	conditional.SetETag(w, etag)
	conditional.SetLastModified(w, resource.ModifiedAt)
	conditional.SetCacheControl(w, conditional.CacheControlOptions{
		Public:         true,
		MaxAge:         300, // Cache for 5 minutes
		MustRevalidate: true,
	})

	respondJSON(w, http.StatusOK, resource)
}

func createResource(w http.ResponseWriter, r *http.Request) {
	var resource Resource
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return
	}

	// Generate ID and timestamps
	resource.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	resource.CreatedAt = time.Now()
	resource.ModifiedAt = time.Now()

	resources[resource.ID] = &resource

	// Set ETag for new resource
	resourceJSON, _ := json.Marshal(resource)
	etag := conditional.DefaultETagGenerator(resourceJSON)
	conditional.SetETag(w, etag)
	conditional.SetLastModified(w, resource.ModifiedAt)

	respondJSON(w, http.StatusCreated, resource)
}

func updateResource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	resource, exists := resources[id]
	if !exists {
		respondError(w, http.StatusNotFound, fmt.Errorf("resource not found"))
		return
	}

	// Marshal current state
	currentJSON, _ := json.Marshal(resource)
	currentETag := conditional.DefaultETagGenerator(currentJSON)

	// Check conditional headers (optimistic concurrency)
	if conditional.CheckConditionalRequest(w, r, currentETag, resource.ModifiedAt) {
		return // Precondition failed or not modified
	}

	// Decode new version
	var updated Resource
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return
	}

	// Preserve system fields
	updated.ID = resource.ID
	updated.CreatedAt = resource.CreatedAt
	updated.ModifiedAt = time.Now()

	resources[id] = &updated

	// Set new ETag
	updatedJSON, _ := json.Marshal(updated)
	newETag := conditional.DefaultETagGenerator(updatedJSON)
	conditional.SetETag(w, newETag)
	conditional.SetLastModified(w, updated.ModifiedAt)

	respondJSON(w, http.StatusOK, updated)
}

func patchResource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	resource, exists := resources[id]
	if !exists {
		respondError(w, http.StatusNotFound, fmt.Errorf("resource not found"))
		return
	}

	// Marshal current state
	currentJSON, _ := json.Marshal(resource)
	currentETag := conditional.DefaultETagGenerator(currentJSON)

	// Check conditional headers
	condInfo := conditional.ExtractConditionalInfo(r)
	if condInfo.IfMatch != "" {
		if !conditional.MatchesETag(condInfo.IfMatch, currentETag) {
			respondError(w, http.StatusPreconditionFailed, fmt.Errorf("ETag mismatch"))
			return
		}
	}

	// Read patch document
	patchData := make([]byte, 0)
	if r.Body != nil {
		var err error
		patchData, err = json.Marshal(r.Body)
		if err != nil {
			buf := make([]byte, r.ContentLength)
			r.Body.Read(buf)
			patchData = buf
		}
	}

	// Detect patch type from Content-Type
	patchType := patch.DetectPatchType(r.Header.Get("Content-Type"))

	fmt.Printf("Applying patch type: %s\n", patchType)
	fmt.Printf("Patch data: %s\n", string(patchData))

	// Validate JSON Patch if applicable
	if patchType == patch.JSONPatch {
		if err := patch.ValidateJSONPatch(patchData); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON Patch: %w", err))
			return
		}
	}

	// Apply patch
	updatedJSON, err := patch.ApplyPatch(currentJSON, patchData, patchType)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, fmt.Errorf("failed to apply patch: %w", err))
		return
	}

	// Unmarshal back to resource
	var updated Resource
	if err := json.Unmarshal(updatedJSON, &updated); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal updated resource: %w", err))
		return
	}

	// Preserve system fields
	updated.ID = resource.ID
	updated.CreatedAt = resource.CreatedAt
	updated.ModifiedAt = time.Now()

	resources[id] = &updated

	// Set new ETag
	finalJSON, _ := json.Marshal(updated)
	newETag := conditional.DefaultETagGenerator(finalJSON)
	conditional.SetETag(w, newETag)
	conditional.SetLastModified(w, updated.ModifiedAt)

	// Include changed paths in response header for debugging
	changes, _ := patch.ComputePatchChanges(currentJSON, finalJSON)
	w.Header().Set("X-Patch-Changes", fmt.Sprintf("%v", changes))

	respondJSON(w, http.StatusOK, updated)
}

func deleteResource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	resource, exists := resources[id]
	if !exists {
		respondError(w, http.StatusNotFound, fmt.Errorf("resource not found"))
		return
	}

	// Check conditional headers before deleting
	resourceJSON, _ := json.Marshal(resource)
	etag := conditional.DefaultETagGenerator(resourceJSON)

	if conditional.CheckConditionalRequest(w, r, etag, resource.ModifiedAt) {
		return // Precondition failed
	}

	delete(resources, id)

	w.WriteHeader(http.StatusNoContent)
}

func optionsResource(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.Header().Set("Accept-Patch", string(patch.JSONMergePatch))
	w.Header().Add("Accept-Patch", string(patch.JSONPatch))
	w.Header().Add("Accept-Patch", string(patch.ShorthandPatch))
	w.WriteHeader(http.StatusOK)
}

// Helper functions
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}

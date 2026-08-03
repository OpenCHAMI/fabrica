// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"sync"
	"time"
)

// CachedResult holds parsed annotations with metadata
//
// Annotations is keyed by resource type name and holds the complete
// *ResourceAnnotations. It previously stored only the flattened field map,
// which silently dropped resource-level state (IsResource, StorageMode, and
// now composite indexes and migration policy) on every cache hit.
type CachedResult struct {
	Annotations map[string]*ResourceAnnotations
	ModTime     time.Time
	ParseTime   time.Time
}

// AnnotationCache caches parsed annotations to avoid re-parsing unchanged files
type AnnotationCache struct {
	mutex sync.RWMutex
	cache map[string]*CachedResult
}

// NewAnnotationCache creates a new cache
func NewAnnotationCache() *AnnotationCache {
	return &AnnotationCache{
		cache: make(map[string]*CachedResult),
	}
}

// Get retrieves cached annotations if the file hasn't been modified
func (c *AnnotationCache) Get(filename string) (map[string]*ResourceAnnotations, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	cached, ok := c.cache[filename]
	if !ok {
		return nil, false
	}

	// Check if file was modified since cache
	stat, err := os.Stat(filename)
	if err != nil || stat.ModTime().After(cached.ModTime) {
		return nil, false
	}

	return cached.Annotations, true
}

// Set stores parsed annotations in cache
func (c *AnnotationCache) Set(filename string, anns map[string]*ResourceAnnotations) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	stat, err := os.Stat(filename)
	modTime := time.Now()
	if err == nil {
		modTime = stat.ModTime()
	}

	c.cache[filename] = &CachedResult{
		Annotations: anns,
		ModTime:     modTime,
		ParseTime:   time.Now(),
	}
}

// Invalidate removes a file from cache
func (c *AnnotationCache) Invalidate(filename string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.cache, filename)
}

// Clear removes all entries from cache
func (c *AnnotationCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache = make(map[string]*CachedResult)
}

// Size returns the number of cached files
func (c *AnnotationCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.cache)
}

// Global cache instance
var globalCache = NewAnnotationCache()

// GetGlobalCache returns the global cache instance
func GetGlobalCache() *AnnotationCache {
	return globalCache
}

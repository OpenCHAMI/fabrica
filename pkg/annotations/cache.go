// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	Annotations  map[string]*ResourceAnnotations
	ModTime      time.Time
	Dependencies map[string]time.Time
	PackageFiles []string // sorted list of Go source files in the package at parse time
	ParseTime    time.Time
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

	// Check every tracked dependency's modification time
	for dependency, modTime := range cached.Dependencies {
		stat, err := os.Stat(dependency)
		if err != nil || stat.ModTime().After(modTime) {
			return nil, false
		}
	}

	// Check whether the package file set has changed (files added or removed)
	currentFiles, err := packageFiles(filename)
	if err != nil {
		// Cannot enumerate; conservatively invalidate
		return nil, false
	}
	if !equalStringSlices(cached.PackageFiles, currentFiles) {
		return nil, false
	}

	return cached.Annotations, true
}

// packageFiles returns a sorted list of Go source file paths in the same package
// as filename, excluding test files and the filename itself.
func packageFiles(filename string) ([]string, error) {
	dir := filepath.Dir(filename)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	base := filepath.Base(filename)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if name == base {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}

// Set stores parsed annotations in cache
func (c *AnnotationCache) Set(filename string, anns map[string]*ResourceAnnotations) {
	c.SetWithDependencies(filename, anns, nil)
}

// SetWithDependencies stores parsed annotations with package dependency mtimes.
func (c *AnnotationCache) SetWithDependencies(filename string, anns map[string]*ResourceAnnotations, dependencies []string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	stat, err := os.Stat(filename)
	modTime := time.Now()
	if err == nil {
		modTime = stat.ModTime()
	}

	dependencyTimes := make(map[string]time.Time, len(dependencies))
	for _, dependency := range dependencies {
		stat, err := os.Stat(dependency)
		if err == nil {
			dependencyTimes[dependency] = stat.ModTime()
		}
		// If stat fails, omit the dependency; Get will see a stat error and invalidate.
	}

	// Defend against missing entries on stat errors: record every dependency so
	// that any future stat error on a previously-missing file triggers invalidation.
	// Sort to ensure a deterministic order for comparison.
	sortedDeps := make([]string, len(dependencies))
	copy(sortedDeps, dependencies)
	sort.Strings(sortedDeps)

	c.cache[filename] = &CachedResult{
		Annotations:  anns,
		ModTime:      modTime,
		Dependencies: dependencyTimes,
		PackageFiles: sortedDeps,
		ParseTime:    time.Now(),
	}
}

// equalStringSlices reports whether a and b contain the same elements in the same order.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

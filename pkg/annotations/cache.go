// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"crypto/sha256"
	"os"
	"sync"
	"time"
)

// CachedResult holds parsed annotations with metadata
type CachedResult struct {
	Annotations map[string]*FieldAnnotations
	Resources   map[string]*ResourceAnnotations
	ModTime     time.Time
	ParseTime   time.Time
	contentID   [sha256.Size]byte
}

// AnnotationCache caches parsed annotations to avoid re-parsing unchanged files
type AnnotationCache struct {
	mutex   sync.RWMutex
	cache   map[string]*CachedResult
	storage map[resolvedCacheKey]*resolvedCachedResult
}

type resolvedCacheKey struct {
	filename     string
	resourceName string
	dialect      Dialect
}

type resolvedCachedResult struct {
	storage   *ResolvedResourceStorage
	contentID [sha256.Size]byte
}

// NewAnnotationCache creates a new cache
func NewAnnotationCache() *AnnotationCache {
	return &AnnotationCache{
		cache: make(map[string]*CachedResult),
	}
}

// Get retrieves cached annotations if the file hasn't been modified
func (c *AnnotationCache) Get(filename string) (map[string]*FieldAnnotations, bool) {
	source, err := os.ReadFile(filename)
	if err != nil {
		c.Invalidate(filename)
		return nil, false
	}
	cached, ok := c.lookup(filename, sha256.Sum256(source))
	if !ok || cached.Annotations == nil {
		return nil, false
	}
	return cloneFieldAnnotationsMap(cached.Annotations), true
}

// Set stores parsed annotations in cache
func (c *AnnotationCache) Set(filename string, anns map[string]*FieldAnnotations) {
	source, readErr := os.ReadFile(filename)
	contentID := [sha256.Size]byte{}
	if readErr == nil {
		contentID = sha256.Sum256(source)
	}
	modTime := time.Now()
	if stat, statErr := os.Stat(filename); statErr == nil {
		modTime = stat.ModTime()
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.cache == nil {
		c.cache = make(map[string]*CachedResult)
	}
	c.cache[filename] = &CachedResult{
		Annotations: cloneFieldAnnotationsMap(anns),
		ModTime:     modTime,
		ParseTime:   time.Now(),
		contentID:   contentID,
	}
}

func (c *AnnotationCache) getResources(filename string, source []byte) (map[string]*ResourceAnnotations, bool) {
	cached, ok := c.lookup(filename, sha256.Sum256(source))
	if !ok || cached.Resources == nil {
		return nil, false
	}
	return cloneResourceAnnotationsMap(cached.Resources), true
}

func (c *AnnotationCache) setResources(filename string, source []byte, resources map[string]*ResourceAnnotations) {
	modTime := time.Now()
	if stat, err := os.Stat(filename); err == nil {
		modTime = stat.ModTime()
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.cache == nil {
		c.cache = make(map[string]*CachedResult)
	}
	c.cache[filename] = &CachedResult{
		Resources: cloneResourceAnnotationsMap(resources),
		ModTime:   modTime,
		ParseTime: time.Now(),
		contentID: sha256.Sum256(source),
	}
}

func (c *AnnotationCache) getStorage(
	key resolvedCacheKey,
	source []byte,
) (*ResolvedResourceStorage, bool) {
	contentID := sha256.Sum256(source)
	c.mutex.Lock()
	defer c.mutex.Unlock()
	cached, ok := c.storage[key]
	if !ok {
		return nil, false
	}
	if cached.contentID != contentID {
		c.invalidateLocked(key.filename)
		return nil, false
	}
	return cloneResolvedResourceStorage(cached.storage), true
}

func (c *AnnotationCache) setStorage(
	key resolvedCacheKey,
	source []byte,
	storage *ResolvedResourceStorage,
) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.storage == nil {
		c.storage = make(map[resolvedCacheKey]*resolvedCachedResult)
	}
	c.storage[key] = &resolvedCachedResult{
		storage:   cloneResolvedResourceStorage(storage),
		contentID: sha256.Sum256(source),
	}
}

func (c *AnnotationCache) lookup(filename string, contentID [sha256.Size]byte) (*CachedResult, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	cached, ok := c.cache[filename]
	if !ok {
		return nil, false
	}
	if cached.contentID != contentID {
		c.invalidateLocked(filename)
		return nil, false
	}
	return cached, true
}

// Invalidate removes a file from cache
func (c *AnnotationCache) Invalidate(filename string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.invalidateLocked(filename)
}

func (c *AnnotationCache) invalidateLocked(filename string) {
	delete(c.cache, filename)
	for key := range c.storage {
		if key.filename == filename {
			delete(c.storage, key)
		}
	}
}

// Clear removes all entries from cache
func (c *AnnotationCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache = make(map[string]*CachedResult)
	c.storage = make(map[resolvedCacheKey]*resolvedCachedResult)
}

// Size returns the number of cached files
func (c *AnnotationCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.cache) + len(c.storage)
}

// Global cache instance
var globalCache = NewAnnotationCache()

// GetGlobalCache returns the global cache instance
func GetGlobalCache() *AnnotationCache {
	return globalCache
}

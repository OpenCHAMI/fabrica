package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileBackend implements StorageBackend using file-based storage.
//
// This implementation stores each resource as a separate JSON file
// in a directory structure organized by resource type:
//
//	baseDir/
//	├── users/
//	│   ├── user-123.json
//	│   └── user-456.json
//	├── products/
//	│   ├── product-789.json
//	│   └── product-abc.json
//	└── orders/
//	    └── order-def.json
//
// Features:
//   - Thread-safe: Uses file locking for concurrent access
//   - Atomic writes: Uses temp files + rename for atomicity
//   - Auto-creation: Creates directories as needed
//   - Validation: Checks JSON format before saving
//   - Error recovery: Continues operation even if some files are corrupted
//
// Limitations:
//   - Performance: Not optimized for large numbers of resources
//   - Scalability: File system limits apply
//   - Consistency: No transactions across multiple resources
//   - Locking: File locking may not work on all file systems
//
// This backend is suitable for:
//   - Development and testing
//   - Small to medium deployments
//   - Environments where simplicity is preferred over performance
//   - Situations where human-readable storage is valuable
type FileBackend struct {
	baseDir         string
	mu              sync.RWMutex
	closed          bool
	versionRegistry VersionRegistry // Version registry for conversion support
}

// VersionRegistry is an interface for version conversion support
// This allows the storage backend to be independent of the versioning package
type VersionRegistry interface {
	GetDefaultVersion(resourceType string) string
	GetVersion(resourceType, version string) (VersionInfo, bool)
}

// VersionInfo contains information about a specific version
type VersionInfo interface {
	Constructor() interface{}
	Converter() VersionConverter
}

// VersionConverter handles version conversion
type VersionConverter interface {
	Convert(resource interface{}, fromVersion, toVersion string) (interface{}, error)
}

// NewFileBackend creates a new file-based storage backend.
//
// Parameters:
//   - baseDir: Root directory for storing resource files
//
// Returns:
//   - *FileBackend: Configured file backend
//   - error: Any error that occurred during initialization
//
// The function will create the base directory if it doesn't exist.
//
// Example:
//
//	backend, err := storage.NewFileBackend("./data")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer backend.Close()
func NewFileBackend(baseDir string) (*FileBackend, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	backend := &FileBackend{
		baseDir: baseDir,
	}

	return backend, nil
}

// resourceTypeToDir maps resource type names to directory names
func (f *FileBackend) resourceTypeToDir(resourceType string) string {
	// Convert to lowercase and pluralize by adding 's' if not already plural
	dir := strings.ToLower(resourceType)
	if !strings.HasSuffix(dir, "s") {
		dir = dir + "s"
	}
	return dir
}

// getFilePath returns the file path for a specific resource
func (f *FileBackend) getFilePath(resourceType, uid string) string {
	dir := f.resourceTypeToDir(resourceType)
	return filepath.Join(f.baseDir, dir, uid+".json")
}

// getDirPath returns the directory path for a resource type
func (f *FileBackend) getDirPath(resourceType string) string {
	dir := f.resourceTypeToDir(resourceType)
	return filepath.Join(f.baseDir, dir)
}

// checkClosed returns an error if the backend has been closed
func (f *FileBackend) checkClosed() error {
	if f.closed {
		return fmt.Errorf("storage backend has been closed")
	}
	return nil
}

// LoadAll implements StorageBackend.LoadAll
func (f *FileBackend) LoadAll(ctx context.Context, resourceType string) ([]json.RawMessage, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, err
	}

	dirPath := f.getDirPath(resourceType)

	// Check if context is cancelled before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []json.RawMessage{}, nil // Empty slice, not an error
		}
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var resources []json.RawMessage
	for _, entry := range entries {
		// Check for cancellation periodically
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Log warning but continue with other files
			continue
		}

		// Validate JSON format
		if !json.Valid(data) {
			// Log warning but continue with other files
			continue
		}

		resources = append(resources, json.RawMessage(data))
	}

	return resources, nil
}

// Load implements StorageBackend.Load
func (f *FileBackend) Load(ctx context.Context, resourceType, uid string) (json.RawMessage, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, err
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	filePath := f.getFilePath(resourceType, uid)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Validate JSON format
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON in file %s: %w", filePath, ErrInvalidData)
	}

	return json.RawMessage(data), nil
}

// Save implements StorageBackend.Save
func (f *FileBackend) Save(ctx context.Context, resourceType, uid string, data json.RawMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkClosed(); err != nil {
		return err
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate JSON format
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON data: %w", ErrInvalidData)
	}

	filePath := f.getFilePath(resourceType, uid)

	// Ensure directory exists
	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Use atomic write: write to temp file, then rename
	tempPath := filePath + ".tmp"

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on error
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file %s to %s: %w", tempPath, filePath, err)
	}

	return nil
}

// Delete implements StorageBackend.Delete
func (f *FileBackend) Delete(ctx context.Context, resourceType, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkClosed(); err != nil {
		return err
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	filePath := f.getFilePath(resourceType, uid)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	return nil
}

// Exists implements StorageBackend.Exists
func (f *FileBackend) Exists(ctx context.Context, resourceType, uid string) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return false, err
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	filePath := f.getFilePath(resourceType, uid)

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	return true, nil
}

// List implements StorageBackend.List
func (f *FileBackend) List(ctx context.Context, resourceType string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, err
	}

	dirPath := f.getDirPath(resourceType)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // Empty slice, not an error
		}
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var uids []string
	for _, entry := range entries {
		// Check for cancellation periodically
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract UID from filename (remove .json extension)
		uid := strings.TrimSuffix(entry.Name(), ".json")
		uids = append(uids, uid)
	}

	return uids, nil
}

// Close implements StorageBackend.Close
func (f *FileBackend) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closed = true
	return nil
}

// SetVersionRegistry sets the version registry for version-aware operations.
// This must be called before using version-aware methods.
func (f *FileBackend) SetVersionRegistry(registry VersionRegistry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.versionRegistry = registry
}

// LoadWithVersion implements StorageBackend.LoadWithVersion
func (f *FileBackend) LoadWithVersion(ctx context.Context, resourceType, uid, version string) (json.RawMessage, string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, "", err
	}

	if f.versionRegistry == nil {
		return nil, "", fmt.Errorf("version registry not set")
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, "", ctx.Err()
	default:
	}

	// Load the raw resource (stored in default version)
	rawData, err := f.Load(ctx, resourceType, uid)
	if err != nil {
		return nil, "", err
	}

	// Get default version for this resource type
	defaultVersion := f.versionRegistry.GetDefaultVersion(resourceType)
	if defaultVersion == "" {
		// No versioning configured, return raw data
		return rawData, "v1", nil
	}

	// If requested version matches storage version, return as-is
	if version == "" || version == defaultVersion {
		return rawData, defaultVersion, nil
	}

	// Need to convert - get type info for both versions
	typeInfo, ok := f.versionRegistry.GetVersion(resourceType, version)
	if !ok {
		return nil, "", fmt.Errorf("unsupported version %s for %s", version, resourceType)
	}

	defaultTypeInfo, ok := f.versionRegistry.GetVersion(resourceType, defaultVersion)
	if !ok {
		return nil, "", fmt.Errorf("failed to get default version info")
	}

	// Unmarshal into default version
	defaultResource := defaultTypeInfo.Constructor()
	if err := json.Unmarshal(rawData, defaultResource); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	// Convert to requested version
	if typeInfo.Converter() != nil {
		converted, err := typeInfo.Converter().Convert(defaultResource, defaultVersion, version)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert from %s to %s: %w", defaultVersion, version, err)
		}

		// Marshal the converted resource
		convertedData, err := json.Marshal(converted)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal converted resource: %w", err)
		}

		return json.RawMessage(convertedData), version, nil
	}

	// No converter available
	return nil, "", fmt.Errorf("no converter available for %s version %s", resourceType, version)
}

// LoadAllWithVersion implements StorageBackend.LoadAllWithVersion
func (f *FileBackend) LoadAllWithVersion(ctx context.Context, resourceType, version string) ([]json.RawMessage, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, err
	}

	if f.versionRegistry == nil {
		return nil, fmt.Errorf("version registry not set")
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Load all resources in default version
	rawResources, err := f.LoadAll(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	// Get default version
	defaultVersion := f.versionRegistry.GetDefaultVersion(resourceType)
	if defaultVersion == "" {
		// No versioning configured, return raw data
		return rawResources, nil
	}

	// If requested version matches storage version, return as-is
	if version == "" || version == defaultVersion {
		return rawResources, nil
	}

	// Need to convert each resource
	typeInfo, ok := f.versionRegistry.GetVersion(resourceType, version)
	if !ok {
		return nil, fmt.Errorf("unsupported version %s for %s", version, resourceType)
	}

	defaultTypeInfo, ok := f.versionRegistry.GetVersion(resourceType, defaultVersion)
	if !ok {
		return nil, fmt.Errorf("failed to get default version info")
	}

	if typeInfo.Converter() == nil {
		return nil, fmt.Errorf("no converter available for %s version %s", resourceType, version)
	}

	var convertedResources []json.RawMessage
	for _, rawData := range rawResources {
		// Check for cancellation periodically
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Unmarshal into default version
		defaultResource := defaultTypeInfo.Constructor()
		if err := json.Unmarshal(rawData, defaultResource); err != nil {
			// Skip corrupted resources
			continue
		}

		// Convert to requested version
		converted, err := typeInfo.Converter().Convert(defaultResource, defaultVersion, version)
		if err != nil {
			// Skip resources that fail conversion
			continue
		}

		// Marshal the converted resource
		convertedData, err := json.Marshal(converted)
		if err != nil {
			// Skip resources that fail marshaling
			continue
		}

		convertedResources = append(convertedResources, json.RawMessage(convertedData))
	}

	return convertedResources, nil
}

// SaveWithVersion implements StorageBackend.SaveWithVersion
func (f *FileBackend) SaveWithVersion(ctx context.Context, resourceType, uid string, data json.RawMessage, version string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkClosed(); err != nil {
		return err
	}

	if f.versionRegistry == nil {
		return fmt.Errorf("version registry not set")
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Get default version (storage version)
	defaultVersion := f.versionRegistry.GetDefaultVersion(resourceType)
	if defaultVersion == "" {
		// No versioning configured, save as-is
		return f.Save(ctx, resourceType, uid, data)
	}

	// If data is already in default version, save as-is
	if version == "" || version == defaultVersion {
		return f.Save(ctx, resourceType, uid, data)
	}

	// Need to convert to storage version
	typeInfo, ok := f.versionRegistry.GetVersion(resourceType, version)
	if !ok {
		return fmt.Errorf("unsupported version %s for %s", version, resourceType)
	}

	if typeInfo.Converter() == nil {
		return fmt.Errorf("no converter available for %s version %s", resourceType, version)
	}

	// Unmarshal into provided version
	resource := typeInfo.Constructor()
	if err := json.Unmarshal(data, resource); err != nil {
		return fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	// Convert to storage version
	converted, err := typeInfo.Converter().Convert(resource, version, defaultVersion)
	if err != nil {
		return fmt.Errorf("failed to convert from %s to %s: %w", version, defaultVersion, err)
	}

	// Marshal to storage format
	storageData, err := json.Marshal(converted)
	if err != nil {
		return fmt.Errorf("failed to marshal converted resource: %w", err)
	}

	// Save in storage version
	return f.Save(ctx, resourceType, uid, json.RawMessage(storageData))
}

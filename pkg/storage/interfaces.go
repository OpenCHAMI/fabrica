// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package storage provides storage interfaces and implementations for framework resources.
//
// The storage system is designed to be pluggable, allowing different backends
// (file-based, database, cloud storage) to be used without changing the rest
// of the application.
//
// Architecture:
//   - StorageBackend: Main interface for CRUD operations
//   - ResourceStorage: Type-safe operations for specific resource types
//   - FileStorage: File-based implementation (default)
//   - Future: DatabaseStorage, CloudStorage, etc.
//
// Usage:
//
//	// Use the default file-based storage
//	backend := storage.NewFileBackend("./data")
//
//	// Get type-safe storage for any resource type
//	resourceStorage := storage.NewResourceStorage[*MyResource](backend, "MyResource")
//
//	// Perform operations
//	resources, err := resourceStorage.LoadAll(ctx)
//	resource, err := resourceStorage.Load(ctx, "resource-123")
//	err = resourceStorage.Save(ctx, resource)
//	err = resourceStorage.Delete(ctx, "resource-123")
//
// Thread Safety:
//
//	Storage implementations should be safe for concurrent use.
//	File-based storage uses file locking where necessary.
//
// Error Handling:
//
//	All operations return structured errors that can be checked:
//	- ErrNotFound: Resource doesn't exist
//	- ErrAlreadyExists: Resource already exists (for Create operations)
//	- ErrInvalidData: Data validation failed
//	- Backend-specific errors (e.g., file permissions, network issues)
package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

// Common storage errors
var (
	ErrNotFound      = fmt.Errorf("resource not found")
	ErrAlreadyExists = fmt.Errorf("resource already exists")
	ErrInvalidData   = fmt.Errorf("invalid data")
)

// StorageBackend defines the core storage operations that any storage implementation must provide.
//
// All storage backends must implement these operations for any resource type.
// The interface is designed to be:
//   - Type-agnostic: Works with any resource that can be marshaled/unmarshaled
//   - Context-aware: Supports cancellation and timeouts
//   - Error-rich: Provides detailed error information
//   - Extensible: Easy to add new operations
//
// Implementation Requirements:
//   - Thread-safe: Multiple goroutines can use the same backend safely
//   - Atomic operations: Save/Delete operations should be atomic where possible
//   - Consistent: Operations should be consistent across multiple calls
//   - Resilient: Should handle and recover from transient failures
//
// Resource Identification:
//
//	Resources are identified by their UID (unique identifier).
//	The resource type is determined by the resourceType parameter.
//
// Data Format:
//
//	Resources are stored as JSON-serializable data.
//	The backend is responsible for serialization/deserialization.
//
//nolint:revive // "StorageBackend" name is intentional; "Backend" alone would be ambiguous
type StorageBackend interface {
	// LoadAll retrieves all resources of the specified type.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//
	// Returns:
	//   - []json.RawMessage: Array of serialized resources
	//   - error: Any error that occurred during loading
	//
	// Behavior:
	//   - Returns empty slice if no resources exist (not an error)
	//   - Skips corrupted resources and logs warnings
	//   - Respects context cancellation
	//
	// Example:
	//   rawResources, err := backend.LoadAll(ctx, "User")
	LoadAll(ctx context.Context, resourceType string) ([]json.RawMessage, error)

	// Load retrieves a single resource by UID.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//   - uid: Unique identifier of the resource
	//
	// Returns:
	//   - json.RawMessage: Serialized resource data
	//   - error: ErrNotFound if resource doesn't exist, other errors for failures
	//
	// Behavior:
	//   - Returns ErrNotFound if resource doesn't exist
	//   - Validates UID format before attempting load
	//   - Respects context cancellation
	//
	// Example:
	//   rawUser, err := backend.Load(ctx, "User", "user-123")
	//   if errors.Is(err, storage.ErrNotFound) {
	//       // Handle missing resource
	//   }
	Load(ctx context.Context, resourceType, uid string) (json.RawMessage, error)

	// Save stores a resource, creating or updating as needed.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//   - uid: Unique identifier of the resource
	//   - data: Serialized resource data
	//
	// Returns:
	//   - error: Any error that occurred during saving
	//
	// Behavior:
	//   - Creates resource if it doesn't exist
	//   - Updates resource if it already exists
	//   - Validates data format before saving
	//   - Operation should be atomic where possible
	//   - Respects context cancellation
	//
	// Example:
	//   data, _ := json.Marshal(user)
	//   err := backend.Save(ctx, "User", user.GetUID(), data)
	Save(ctx context.Context, resourceType, uid string, data json.RawMessage) error

	// Delete removes a resource by UID.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//   - uid: Unique identifier of the resource
	//
	// Returns:
	//   - error: ErrNotFound if resource doesn't exist, other errors for failures
	//
	// Behavior:
	//   - Returns ErrNotFound if resource doesn't exist
	//   - Operation should be atomic where possible
	//   - Cleanup any related data (indexes, caches, etc.)
	//   - Respects context cancellation
	//
	// Example:
	//   err := backend.Delete(ctx, "User", "user-123")
	//   if errors.Is(err, storage.ErrNotFound) {
	//       // Resource was already deleted
	//   }
	Delete(ctx context.Context, resourceType, uid string) error

	// Exists checks if a resource exists without loading it.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//   - uid: Unique identifier of the resource
	//
	// Returns:
	//   - bool: true if resource exists, false otherwise
	//   - error: Any error that occurred during the check
	//
	// Behavior:
	//   - More efficient than Load for existence checks
	//   - Should not return ErrNotFound (use return value instead)
	//   - Respects context cancellation
	//
	// Example:
	//   exists, err := backend.Exists(ctx, "User", "user-123")
	//   if err != nil {
	//       // Handle error
	//   } else if !exists {
	//       // Resource doesn't exist
	//   }
	Exists(ctx context.Context, resourceType, uid string) (bool, error)

	// List returns UIDs of all resources of the specified type.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//
	// Returns:
	//   - []string: Array of resource UIDs
	//   - error: Any error that occurred during listing
	//
	// Behavior:
	//   - Returns empty slice if no resources exist (not an error)
	//   - More efficient than LoadAll for getting UIDs only
	//   - Respects context cancellation
	//
	// Example:
	//   uids, err := backend.List(ctx, "User")
	//   fmt.Printf("Found %d Users\n", len(uids))
	List(ctx context.Context, resourceType string) ([]string, error)

	// Close releases any resources held by the backend.
	//
	// Returns:
	//   - error: Any error that occurred during cleanup
	//
	// Behavior:
	//   - Should be called when the backend is no longer needed
	//   - Should be safe to call multiple times
	//   - May block to ensure data consistency
	//   - After Close(), other operations may return errors
	//
	// Example:
	//   defer backend.Close()
	Close() error

	// LoadWithVersion retrieves a single resource by UID and returns it in the requested version.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//   - uid: Unique identifier of the resource
	//   - version: Requested schema version (e.g., "v1", "v2beta1")
	//
	// Returns:
	//   - json.RawMessage: Serialized resource data in requested version
	//   - string: Actual version returned (may differ from requested if conversion occurred)
	//   - error: ErrNotFound if resource doesn't exist, error if version not supported
	//
	// Behavior:
	//   - Returns ErrNotFound if resource doesn't exist
	//   - Returns error if requested version is not supported
	//   - May perform version conversion if resource is stored in different version
	//   - Respects context cancellation
	//
	// Example:
	//   rawUser, version, err := backend.LoadWithVersion(ctx, "User", "user-123", "v2beta1")
	LoadWithVersion(ctx context.Context, resourceType, uid, version string) (json.RawMessage, string, error)

	// LoadAllWithVersion retrieves all resources of the specified type in the requested version.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//   - version: Requested schema version (e.g., "v1", "v2beta1")
	//
	// Returns:
	//   - []json.RawMessage: Array of serialized resources in requested version
	//   - error: Error if version not supported
	//
	// Behavior:
	//   - Returns empty slice if no resources exist (not an error)
	//   - Returns error if requested version is not supported
	//   - May perform version conversion on each resource
	//   - Skips corrupted resources and logs warnings
	//   - Respects context cancellation
	//
	// Example:
	//   rawResources, err := backend.LoadAllWithVersion(ctx, "User", "v2beta1")
	LoadAllWithVersion(ctx context.Context, resourceType, version string) ([]json.RawMessage, error)

	// SaveWithVersion stores a resource with version metadata.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resourceType: Type name (e.g., "User", "Product", "Order")
	//   - uid: Unique identifier of the resource
	//   - data: Serialized resource data
	//   - version: Schema version of the provided data (e.g., "v1", "v2beta1")
	//
	// Returns:
	//   - error: Any error that occurred during saving
	//
	// Behavior:
	//   - Creates resource if it doesn't exist
	//   - Updates resource if it already exists
	//   - May convert to storage version before saving
	//   - Stores version metadata with resource
	//   - Operation should be atomic where possible
	//   - Respects context cancellation
	//
	// Example:
	//   data, _ := json.Marshal(userV2)
	//   err := backend.SaveWithVersion(ctx, "User", user.GetUID(), data, "v2beta1")
	SaveWithVersion(ctx context.Context, resourceType, uid string, data json.RawMessage, version string) error
}

// ResourceStorage provides type-safe storage operations for a specific resource type.
//
// This interface wraps StorageBackend to provide type safety and convenience
// methods for working with specific resource types. Each resource type gets
// its own storage instance.
//
// Type Safety:
//
//	All operations work with strongly-typed resource pointers instead of
//	json.RawMessage, reducing the chance of type-related errors.
//
// Convenience:
//
//	Provides higher-level operations like batch loading, filtering, etc.
//	that are commonly needed but not part of the core backend interface.
//
// Example Usage:
//
//	userStorage := NewResourceStorage[*User](backend, "User")
//	users, err := userStorage.LoadAll(ctx)
//	user, err := userStorage.Load(ctx, "user-123")
type ResourceStorage[T any] interface {
	// LoadAll retrieves all resources of this type.
	//
	// Returns:
	//   - []T: Slice of strongly-typed resources
	//   - error: Any error that occurred during loading
	//
	// Behavior:
	//   - Unmarshals each resource from JSON
	//   - Skips resources that fail to unmarshal (logs warnings)
	//   - Returns empty slice if no resources exist
	LoadAll(ctx context.Context) ([]T, error)

	// Load retrieves a single resource by UID.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - uid: Unique identifier of the resource
	//
	// Returns:
	//   - T: Strongly-typed resource
	//   - error: ErrNotFound if resource doesn't exist
	//
	// Behavior:
	//   - Unmarshals resource from JSON
	//   - Returns ErrNotFound if resource doesn't exist
	//   - Returns ErrInvalidData if unmarshaling fails
	Load(ctx context.Context, uid string) (T, error)

	// Save stores a resource.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resource: Strongly-typed resource to save
	//
	// Returns:
	//   - error: Any error that occurred during saving
	//
	// Behavior:
	//   - Marshals resource to JSON
	//   - Extracts UID from resource
	//   - Creates or updates as needed
	Save(ctx context.Context, resource T) error

	// Delete removes a resource by UID.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - uid: Unique identifier of the resource
	//
	// Returns:
	//   - error: ErrNotFound if resource doesn't exist
	Delete(ctx context.Context, uid string) error

	// Exists checks if a resource exists.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - uid: Unique identifier of the resource
	//
	// Returns:
	//   - bool: true if resource exists
	//   - error: Any error that occurred during the check
	Exists(ctx context.Context, uid string) (bool, error)

	// List returns UIDs of all resources of this type.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//
	// Returns:
	//   - []string: Array of resource UIDs
	//   - error: Any error that occurred during listing
	List(ctx context.Context) ([]string, error)

	// LoadWithVersion retrieves a single resource by UID in the requested version.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - uid: Unique identifier of the resource
	//   - version: Requested schema version (e.g., "v1", "v2beta1")
	//
	// Returns:
	//   - interface{}: Resource in requested version (type may vary by version)
	//   - string: Actual version returned
	//   - error: ErrNotFound if resource doesn't exist, error if version not supported
	//
	// Note: Returns interface{} because the concrete type may differ by version.
	// Callers should type assert to the expected version-specific type.
	LoadWithVersion(ctx context.Context, uid string, version string) (interface{}, string, error)

	// LoadAllWithVersion retrieves all resources in the requested version.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - version: Requested schema version (e.g., "v1", "v2beta1")
	//
	// Returns:
	//   - []interface{}: Slice of resources in requested version
	//   - error: Error if version not supported
	//
	// Note: Returns []interface{} because the concrete type may differ by version.
	// Callers should type assert elements to the expected version-specific type.
	LoadAllWithVersion(ctx context.Context, version string) ([]interface{}, error)

	// SaveWithVersion stores a resource with version metadata.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resource: Resource to save (may be any supported version)
	//   - version: Schema version of the provided resource
	//
	// Returns:
	//   - error: Any error that occurred during saving
	//
	// Behavior:
	//   - Accepts resources in any supported version
	//   - May convert to storage version before persisting
	//   - Stores version metadata
	SaveWithVersion(ctx context.Context, resource interface{}, version string) error
}

// Resource interface defines the minimal interface that all resources must implement
// for storage operations. This allows the storage system to work with any resource
// type without knowing the specific implementation details.
type Resource interface {
	// GetUID returns the unique identifier for this resource.
	// This is used as the primary key for storage operations.
	GetUID() string
}

// resourceStorage is the concrete implementation of ResourceStorage
type resourceStorage[T Resource] struct {
	backend      StorageBackend
	resourceType string
}

// NewResourceStorage creates a new type-safe storage for a specific resource type.
//
// Parameters:
//   - backend: The storage backend to use
//   - resourceType: The name of the resource type (e.g., "User", "Product")
//
// Returns:
//   - ResourceStorage[T]: Type-safe storage interface
//
// Example:
//
//	userStorage := NewResourceStorage[*User](backend, "User")
func NewResourceStorage[T Resource](backend StorageBackend, resourceType string) ResourceStorage[T] {
	return &resourceStorage[T]{
		backend:      backend,
		resourceType: resourceType,
	}
}

// LoadAll implements ResourceStorage.LoadAll
func (s *resourceStorage[T]) LoadAll(ctx context.Context) ([]T, error) {
	rawResources, err := s.backend.LoadAll(ctx, s.resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to load all %s: %w", s.resourceType, err)
	}

	var resources []T
	for _, raw := range rawResources {
		var resource T
		if err := json.Unmarshal(raw, &resource); err != nil {
			// Log warning but continue processing other resources
			continue
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// Load implements ResourceStorage.Load
func (s *resourceStorage[T]) Load(ctx context.Context, uid string) (T, error) {
	var zero T

	raw, err := s.backend.Load(ctx, s.resourceType, uid)
	if err != nil {
		return zero, fmt.Errorf("failed to load %s %s: %w", s.resourceType, uid, err)
	}

	var resource T
	if err := json.Unmarshal(raw, &resource); err != nil {
		return zero, fmt.Errorf("failed to unmarshal %s %s: %w", s.resourceType, uid, ErrInvalidData)
	}

	return resource, nil
}

// Save implements ResourceStorage.Save
func (s *resourceStorage[T]) Save(ctx context.Context, resource T) error {
	data, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", s.resourceType, err)
	}

	uid := resource.GetUID()
	if uid == "" {
		return fmt.Errorf("resource has empty UID: %w", ErrInvalidData)
	}

	if err := s.backend.Save(ctx, s.resourceType, uid, data); err != nil {
		return fmt.Errorf("failed to save %s %s: %w", s.resourceType, uid, err)
	}

	return nil
}

// Delete implements ResourceStorage.Delete
func (s *resourceStorage[T]) Delete(ctx context.Context, uid string) error {
	if err := s.backend.Delete(ctx, s.resourceType, uid); err != nil {
		return fmt.Errorf("failed to delete %s %s: %w", s.resourceType, uid, err)
	}
	return nil
}

// Exists implements ResourceStorage.Exists
func (s *resourceStorage[T]) Exists(ctx context.Context, uid string) (bool, error) {
	exists, err := s.backend.Exists(ctx, s.resourceType, uid)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of %s %s: %w", s.resourceType, uid, err)
	}
	return exists, nil
}

// List implements ResourceStorage.List
func (s *resourceStorage[T]) List(ctx context.Context) ([]string, error) {
	uids, err := s.backend.List(ctx, s.resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to list %s UIDs: %w", s.resourceType, err)
	}
	return uids, nil
}

// LoadWithVersion implements ResourceStorage.LoadWithVersion
func (s *resourceStorage[T]) LoadWithVersion(ctx context.Context, uid string, version string) (interface{}, string, error) {
	rawData, actualVersion, err := s.backend.LoadWithVersion(ctx, s.resourceType, uid, version)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load %s %s (version %s): %w", s.resourceType, uid, version, err)
	}

	// Unmarshal into interface{} - caller must type assert
	var resource interface{}
	if err := json.Unmarshal(rawData, &resource); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal %s %s: %w", s.resourceType, uid, ErrInvalidData)
	}

	return resource, actualVersion, nil
}

// LoadAllWithVersion implements ResourceStorage.LoadAllWithVersion
func (s *resourceStorage[T]) LoadAllWithVersion(ctx context.Context, version string) ([]interface{}, error) {
	rawResources, err := s.backend.LoadAllWithVersion(ctx, s.resourceType, version)
	if err != nil {
		return nil, fmt.Errorf("failed to load all %s (version %s): %w", s.resourceType, version, err)
	}

	var resources []interface{}
	for _, raw := range rawResources {
		var resource interface{}
		if err := json.Unmarshal(raw, &resource); err != nil {
			// Log warning but continue processing other resources
			continue
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// SaveWithVersion implements ResourceStorage.SaveWithVersion
func (s *resourceStorage[T]) SaveWithVersion(ctx context.Context, resource interface{}, version string) error {
	// Extract UID from resource
	// Try to cast to Resource interface
	r, ok := resource.(Resource)
	if !ok {
		return fmt.Errorf("resource does not implement Resource interface: %w", ErrInvalidData)
	}

	uid := r.GetUID()
	if uid == "" {
		return fmt.Errorf("resource has empty UID: %w", ErrInvalidData)
	}

	data, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", s.resourceType, err)
	}

	if err := s.backend.SaveWithVersion(ctx, s.resourceType, uid, data, version); err != nil {
		return fmt.Errorf("failed to save %s %s (version %s): %w", s.resourceType, uid, version, err)
	}

	return nil
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package resource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Resource represents a common structure for all inventory resources.
//
// Resource follows the Kubernetes resource pattern with APIVersion, Kind, Metadata,
// Spec, and Status fields. All concrete resource types embed this struct to inherit
// common functionality.
//
// Fields:
//   - APIVersion: Version of the resource API (e.g., "v1", "v1beta1")
//   - Kind: Type of resource (e.g., "Node", "Device", "Asset")
//   - SchemaVersion: Version of the resource schema for migration purposes
//   - Metadata: Resource metadata including name, UID, labels, annotations, and timestamps
//   - Spec: Desired state of the resource (defined by concrete types)
//   - Status: Observed state of the resource (defined by concrete types)
//
// Example:
//
//	type Device struct {
//	    Resource
//	    Spec   DeviceSpec
//	    Status DeviceStatus
//	}
//
//	device := &Device{
//	    Resource: Resource{
//	        APIVersion: "v1",
//	        Kind: "Device",
//	        SchemaVersion: "1.0",
//	    },
//	}
//	device.Metadata.Initialize("device-001", GenerateUID("dev"))
type Resource struct {
	APIVersion    string      `json:"apiVersion" yaml:"apiVersion"`
	Kind          string      `json:"kind" yaml:"kind"`
	SchemaVersion string      `json:"schemaVersion" yaml:"schemaVersion"`
	Metadata      Metadata    `json:"metadata" yaml:"metadata"`
	Spec          interface{} `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status        interface{} `json:"status,omitempty" yaml:"status,omitempty"`
}

// BaseResource is an alias for Resource to support both naming conventions.
// Use this when embedding resources in your types for clarity.
type BaseResource = Resource

// GetUID returns the unique identifier of the resource.
//
// This is a convenience method for accessing resource.Metadata.UID.
// UIDs are typically structured identifiers and should be unique across the entire system.
func (r *Resource) GetUID() string {
	return r.Metadata.UID
}

// SetUID sets the unique identifier of the resource.
//
// This should typically only be called during resource creation.
// UIDs should be immutable after creation.
func (r *Resource) SetUID(uid string) {
	r.Metadata.UID = uid
}

// GetName returns the resource name.
//
// Names should be human-readable and unique within their scope/namespace.
// Unlike UIDs, names can potentially be changed (though this should be rare).
func (r *Resource) GetName() string {
	return r.Metadata.Name
}

// SetName sets the resource name.
//
// Use caution when changing names as other resources may reference this resource by name.
func (r *Resource) SetName(name string) {
	r.Metadata.Name = name
}

// GetLabels returns a copy of labels to prevent external modification.
//
// Returns a new map containing all labels. Modifying the returned map
// will not affect the original resource. Use SetLabel to modify labels.
//
// Example:
//
//	labels := resource.GetLabels()
//	for key, value := range labels {
//	    fmt.Printf("%s=%s\n", key, value)
//	}
func (r *Resource) GetLabels() map[string]string {
	if r.Metadata.Labels == nil {
		return make(map[string]string)
	}
	labels := make(map[string]string)
	for k, v := range r.Metadata.Labels {
		labels[k] = v
	}
	return labels
}

// SetLabel sets a single label.
//
// Creates the labels map if it doesn't exist. Labels are used for
// selection and grouping of resources.
//
// Example:
//
//	resource.SetLabel("environment", "production")
//	resource.SetLabel("location", "datacenter-01")
func (r *Resource) SetLabel(key, value string) {
	if r.Metadata.Labels == nil {
		r.Metadata.Labels = make(map[string]string)
	}
	r.Metadata.Labels[key] = value
}

// GetLabel gets a single label value.
//
// Returns the label value and a boolean indicating whether the label exists.
// This is the preferred way to check for label existence and retrieve values.
//
// Example:
//
//	if env, exists := resource.GetLabel("environment"); exists {
//	    fmt.Printf("Environment: %s\n", env)
//	}
func (r *Resource) GetLabel(key string) (string, bool) {
	if r.Metadata.Labels == nil {
		return "", false
	}
	value, exists := r.Metadata.Labels[key]
	return value, exists
}

// RemoveLabel removes a label.
//
// Safe to call even if the label doesn't exist or if labels map is nil.
func (r *Resource) RemoveLabel(key string) {
	if r.Metadata.Labels != nil {
		delete(r.Metadata.Labels, key)
	}
}

// HasLabel checks if a label exists with the given key.
//
// Returns true if the label exists, regardless of its value.
//
// Example:
//
//	if resource.HasLabel("critical") {
//	    // Handle critical resource
//	}
func (r *Resource) HasLabel(key string) bool {
	_, exists := r.GetLabel(key)
	return exists
}

// MatchesLabels checks if resource has all specified labels with matching values.
//
// This is useful for label-based selection. Returns true only if all
// labels in the selector map exist on the resource with matching values.
//
// Example:
//
//	selector := map[string]string{
//	    "environment": "production",
//	    "role": "server",
//	}
//	if resource.MatchesLabels(selector) {
//	    // Resource matches the selector
//	}
func (r *Resource) MatchesLabels(selector map[string]string) bool {
	for key, value := range selector {
		if labelValue, exists := r.GetLabel(key); !exists || labelValue != value {
			return false
		}
	}
	return true
}

// GetAnnotations returns a copy of annotations.
//
// Returns a new map containing all annotations. Modifying the returned map
// will not affect the original resource. Use SetAnnotation to modify annotations.
//
// Example:
//
//	annotations := resource.GetAnnotations()
//	if description, exists := annotations["description"]; exists {
//	    fmt.Printf("Description: %s\n", description)
//	}
func (r *Resource) GetAnnotations() map[string]string {
	if r.Metadata.Annotations == nil {
		return make(map[string]string)
	}
	annotations := make(map[string]string)
	for k, v := range r.Metadata.Annotations {
		annotations[k] = v
	}
	return annotations
}

// SetAnnotation sets a single annotation.
//
// Creates the annotations map if it doesn't exist. Annotations are used for
// storing arbitrary metadata that doesn't need to be queryable.
//
// Example:
//
//	resource.SetAnnotation("deployment.notes", "Deployed during maintenance")
//	resource.SetAnnotation("contact.email", "ops@example.com")
func (r *Resource) SetAnnotation(key, value string) {
	if r.Metadata.Annotations == nil {
		r.Metadata.Annotations = make(map[string]string)
	}
	r.Metadata.Annotations[key] = value
}

// GetAnnotation gets a single annotation value.
//
// Returns the annotation value and a boolean indicating whether the annotation exists.
//
// Example:
//
//	if notes, exists := resource.GetAnnotation("deployment.notes"); exists {
//	    fmt.Printf("Notes: %s\n", notes)
//	}
func (r *Resource) GetAnnotation(key string) (string, bool) {
	if r.Metadata.Annotations == nil {
		return "", false
	}
	value, exists := r.Metadata.Annotations[key]
	return value, exists
}

// RemoveAnnotation removes an annotation.
//
// Safe to call even if the annotation doesn't exist or if annotations map is nil.
func (r *Resource) RemoveAnnotation(key string) {
	if r.Metadata.Annotations != nil {
		delete(r.Metadata.Annotations, key)
	}
}

// Touch updates the UpdatedAt timestamp to the current time.
//
// This is useful for marking a resource as recently modified without
// changing its creation timestamp. Should be called whenever the
// resource is modified.
//
// Example:
//
//	resource.SetLabel("status", "updated")
//	resource.Touch() // Mark as recently updated
func (r *Resource) Touch() {
	r.Metadata.UpdatedAt = time.Now()
}

// Age returns how long ago the resource was created.
//
// This is useful for determining the age of resources for cleanup,
// reporting, or lifecycle management.
//
// Example:
//
//	if resource.Age() > 24*time.Hour {
//	    // Resource is older than 24 hours
//	}
func (r *Resource) Age() time.Duration {
	return time.Since(r.Metadata.CreatedAt)
}

// LastUpdated returns how long ago the resource was last updated.
//
// This is useful for determining staleness of resource data or
// for cache invalidation decisions.
//
// Example:
//
//	if resource.LastUpdated() > time.Hour {
//	    // Resource data might be stale
//	}
func (r *Resource) LastUpdated() time.Duration {
	return time.Since(r.Metadata.UpdatedAt)
}

// UID Generation Helpers
//
// These functions provide structured, human-readable UIDs instead of UUIDs.
// The format is: <prefix>-<random-hex-digits>
//
// This makes logs much easier to follow while still providing sufficient entropy
// for uniqueness. Similar to AWS resource identifiers (e.g., i-1234abcd for instances).
//
// Resource prefixes are registered using RegisterResourcePrefix() which should be
// called during package initialization (typically in init() functions).

// resourcePrefixes holds the registered mappings from resource kinds to prefixes.
var resourcePrefixes = make(map[string]string)
var resourcePrefixesMutex sync.RWMutex

// RegisterResourcePrefix registers a prefix for a specific resource type.
//
// This function should be called during package initialization, typically in
// init() functions. It's thread-safe and will panic if the same resource kind
// or prefix is registered twice to catch configuration errors early.
//
// Parameters:
//   - resourceKind: The Kind field of the resource (e.g., "Device", "Asset", "Sensor")
//   - prefix: The short prefix to use (e.g., "dev", "ast", "sen")
//
// Panics:
//   - If resourceKind is already registered
//   - If prefix is already used by another resource kind
//   - If resourceKind or prefix is empty
//   - If prefix contains invalid characters (only lowercase letters and numbers allowed)
//
// Example:
//
//	func init() {
//	    RegisterResourcePrefix("Device", "dev")
//	    RegisterResourcePrefix("Asset", "ast")
//	    RegisterResourcePrefix("Sensor", "sen")
//	}
func RegisterResourcePrefix(resourceKind, prefix string) {
	if resourceKind == "" {
		panic("resource kind cannot be empty")
	}
	if prefix == "" {
		panic("prefix cannot be empty")
	}

	// Validate prefix format - only lowercase letters and numbers
	// nolint:all // Allow simple loop for clarity
	for _, r := range prefix {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			panic(fmt.Sprintf("prefix '%s' contains invalid characters - only lowercase letters and numbers allowed", prefix))
		}
	}

	resourcePrefixesMutex.Lock()
	defer resourcePrefixesMutex.Unlock()

	// Check if resource kind is already registered
	if existingPrefix, exists := resourcePrefixes[resourceKind]; exists {
		panic(fmt.Sprintf("resource kind '%s' is already registered with prefix '%s'", resourceKind, existingPrefix))
	}

	// Check if prefix is already used
	for existingKind, existingPrefix := range resourcePrefixes {
		if existingPrefix == prefix {
			panic(fmt.Sprintf("prefix '%s' is already used by resource kind '%s'", prefix, existingKind))
		}
	}

	resourcePrefixes[resourceKind] = prefix
}

// GetRegisteredPrefixes returns a copy of all registered resource prefixes.
//
// This is useful for debugging, logging, or building tools that need to
// know about all available resource types.
//
// Returns:
//   - A map of resource kinds to their prefixes
//
// Example:
//
//	prefixes := GetRegisteredPrefixes()
//	for kind, prefix := range prefixes {
//	    fmt.Printf("%s -> %s\n", kind, prefix)
//	}
func GetRegisteredPrefixes() map[string]string {
	resourcePrefixesMutex.RLock()
	defer resourcePrefixesMutex.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]string)
	for k, v := range resourcePrefixes {
		result[k] = v
	}
	return result
}

// IsResourceKindRegistered checks if a resource kind has been registered.
//
// This is useful for validation before attempting to generate UIDs.
//
// Example:
//
//	if !IsResourceKindRegistered("CustomResource") {
//	    return fmt.Errorf("resource kind not registered")
//	}
func IsResourceKindRegistered(resourceKind string) bool {
	resourcePrefixesMutex.RLock()
	defer resourcePrefixesMutex.RUnlock()

	_, exists := resourcePrefixes[resourceKind]
	return exists
}

// IsPrefixRegistered checks if a prefix is already in use.
//
// This is useful for validation when implementing custom registration logic.
//
// Example:
//
//	if IsPrefixRegistered("dev") {
//	    // Handle conflict
//	}
func IsPrefixRegistered(prefix string) bool {
	resourcePrefixesMutex.RLock()
	defer resourcePrefixesMutex.RUnlock()

	for _, existingPrefix := range resourcePrefixes {
		if existingPrefix == prefix {
			return true
		}
	}
	return false
}

// GenerateUID creates a structured UID with the specified prefix.
//
// The UID format is: <prefix>-<8-random-hex-digits>
// This provides 32 bits of entropy (4 billion possible values per prefix),
// which should be sufficient for most inventory systems.
//
// Parameters:
//   - prefix: The prefix to use (e.g., "dev", "ast", "sen")
//
// Returns:
//   - A structured UID string (e.g., "dev-1a2b3c4d")
//   - An error if random generation fails
//
// Example:
//
//	uid, err := GenerateUID("dev")
//	if err != nil {
//	    return err
//	}
//	// uid might be "dev-1a2b3c4d"
func GenerateUID(prefix string) (string, error) {
	return GenerateUIDWithLength(prefix, 8)
}

// GenerateUIDWithLength creates a structured UID with the specified prefix and hex length.
//
// This allows for variable entropy levels. Each hex character provides 4 bits
// of entropy, so 8 characters = 32 bits, 12 characters = 48 bits, etc.
//
// Parameters:
//   - prefix: The prefix to use (e.g., "dev", "ast", "sen")
//   - hexLength: Number of hex characters for the random part (must be even)
//
// Returns:
//   - A structured UID string
//   - An error if random generation fails or hexLength is invalid
//
// Example:
//
//	uid, err := GenerateUIDWithLength("dev", 12)
//	// uid might be "dev-1a2b3c4d5e6f"
func GenerateUIDWithLength(prefix string, hexLength int) (string, error) {
	if hexLength <= 0 || hexLength%2 != 0 {
		return "", fmt.Errorf("hexLength must be positive and even, got %d", hexLength)
	}

	// Generate random bytes (hexLength/2 bytes = hexLength hex characters)
	randomBytes := make([]byte, hexLength/2)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	hexString := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("%s-%s", prefix, hexString), nil
}

// GenerateUIDForResource creates a UID for a specific resource type using registered prefixes.
//
// This is a convenience function that looks up the registered prefix for a resource
// type and generates an appropriate UID.
//
// Parameters:
//   - resourceKind: The Kind field of the resource (e.g., "Device", "Asset", "Sensor")
//
// Returns:
//   - A structured UID string using the registered prefix
//   - An error if the resource kind is not registered or random generation fails
//
// Example:
//
//	uid, err := GenerateUIDForResource("Device")
//	// uid might be "dev-1a2b3c4d"
func GenerateUIDForResource(resourceKind string) (string, error) {
	resourcePrefixesMutex.RLock()
	prefix, exists := resourcePrefixes[resourceKind]
	resourcePrefixesMutex.RUnlock()

	if !exists {
		return "", fmt.Errorf("resource kind '%s' is not registered - call RegisterResourcePrefix() first", resourceKind)
	}

	return GenerateUID(prefix)
}

// ParseUID extracts the prefix and random part from a structured UID.
//
// This is useful for validation, logging, or when you need to extract
// the resource type from a UID.
//
// Parameters:
//   - uid: The UID to parse (e.g., "dev-1a2b3c4d")
//
// Returns:
//   - prefix: The prefix part (e.g., "dev")
//   - randomPart: The hex random part (e.g., "1a2b3c4d")
//   - error: If the UID format is invalid
//
// Example:
//
//	prefix, randomPart, err := ParseUID("dev-1a2b3c4d")
//	// prefix = "dev", randomPart = "1a2b3c4d"
func ParseUID(uid string) (prefix, randomPart string, err error) {
	parts := strings.Split(uid, "-")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid UID format: %s (expected format: prefix-hex)", uid)
	}

	prefix = parts[0]
	randomPart = parts[1]

	// Validate that random part is valid hex
	if _, err := hex.DecodeString(randomPart); err != nil {
		return "", "", fmt.Errorf("invalid hex in UID: %s", randomPart)
	}

	return prefix, randomPart, nil
}

// IsValidUID checks if a UID follows the expected structured format.
//
// This validates that the UID has the correct format (prefix-hex) and that
// the hex part is valid hexadecimal.
//
// Example:
//
//	if IsValidUID("dev-1a2b3c4d") {
//	    // UID is valid
//	}
func IsValidUID(uid string) bool {
	_, _, err := ParseUID(uid)
	return err == nil
}

// GetResourceTypeFromUID attempts to determine the resource type from a UID prefix.
//
// This reverse-maps the prefix back to the resource kind. Useful for logging
// and debugging when you only have a UID.
//
// Parameters:
//   - uid: The UID to analyze (e.g., "dev-1a2b3c4d")
//
// Returns:
//   - The resource kind (e.g., "Device") if the prefix is recognized
//   - An error if the UID format is invalid or prefix is not registered
//
// Example:
//
//	resourceType, err := GetResourceTypeFromUID("dev-1a2b3c4d")
//	// resourceType = "Device"
func GetResourceTypeFromUID(uid string) (string, error) {
	prefix, _, err := ParseUID(uid)
	if err != nil {
		return "", err
	}

	resourcePrefixesMutex.RLock()
	defer resourcePrefixesMutex.RUnlock()

	// Reverse lookup in registered prefixes
	for resourceKind, resourcePrefix := range resourcePrefixes {
		if resourcePrefix == prefix {
			return resourceKind, nil
		}
	}

	return "", fmt.Errorf("prefix '%s' is not registered", prefix)
}

// ResourceWithConditions represents a resource that has a Status field with Conditions.
// This interface allows condition management with automatic event publishing.
type ResourceWithConditions interface { //nolint:revive
	GetUID() string
	GetKind() string
	GetConditions() *[]Condition
}

// SetResourceCondition is a helper function for resources that implement ResourceWithConditions.
// It sets a condition and automatically publishes events if configured.
//
// Example:
//
//	// For a Device that implements ResourceWithConditions
//	SetResourceCondition(ctx, device, "Ready", "True", "Healthy", "All systems operational")
func SetResourceCondition(ctx context.Context, resource ResourceWithConditions, conditionType, status, reason, message string) bool {
	conditions := resource.GetConditions()
	return SetConditionWithEvents(ctx, conditions, conditionType, status, reason, message, resource.GetKind(), resource.GetUID())
}

// Package versioning provides version registry for managing multiple schema versions.
//
// The version registry provides a central location for managing all resource
// schema versions, enabling dynamic version negotiation and conversion.
package versioning

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// SchemaVersion represents metadata about a specific schema version
type SchemaVersion struct {
	Version    string   // "v1", "v2beta1", "v3alpha1"
	IsDefault  bool     // Default version for this resource
	Stability  string   // "stable", "beta", "alpha"
	Deprecated bool     // Mark deprecated versions
	SpecType   string   // Version-specific spec type
	StatusType string   // Version-specific status type
	TypeName   string   // Full type name with package prefix
	Package    string   // Package import path
	Transforms []string // Transformation function names
}

// ResourceTypeInfo contains type information for a specific resource version
type ResourceTypeInfo struct {
	Type        reflect.Type       // Go type for this version
	Constructor func() interface{} // Creates new instance
	Converter   VersionConverter   // Converts between versions
	Metadata    SchemaVersion      // Version metadata
}

// VersionConverter enables conversion between different schema versions
type VersionConverter interface {
	// CanConvert checks if conversion is supported between versions
	CanConvert(fromVersion, toVersion string) bool

	// Convert transforms a resource from one version to another
	Convert(resource interface{}, fromVersion, toVersion string) (interface{}, error)

	// ConvertSpec transforms just the spec portion
	ConvertSpec(spec interface{}, fromVersion, toVersion string) (interface{}, error)

	// ConvertStatus transforms just the status portion
	ConvertStatus(status interface{}, fromVersion, toVersion string) (interface{}, error)
}

// VersionRegistry manages all resource schema versions
type VersionRegistry struct {
	mu        sync.RWMutex
	resources map[string]map[string]ResourceTypeInfo // kind -> version -> info
	defaults  map[string]string                      // kind -> default version
}

// NewVersionRegistry creates a new version registry
func NewVersionRegistry() *VersionRegistry {
	return &VersionRegistry{
		resources: make(map[string]map[string]ResourceTypeInfo),
		defaults:  make(map[string]string),
	}
}

// RegisterVersion adds a new resource version to the registry
func (vr *VersionRegistry) RegisterVersion(kind, version string, info ResourceTypeInfo) error {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	// Validate version format
	if !isValidVersion(version) {
		return fmt.Errorf("invalid version format: %s (expected v1, v2beta1, v3alpha1, etc.)", version)
	}

	// Initialize kind map if needed
	if vr.resources[kind] == nil {
		vr.resources[kind] = make(map[string]ResourceTypeInfo)
	}

	// Check for duplicate registration
	if _, exists := vr.resources[kind][version]; exists {
		return fmt.Errorf("version %s for kind %s already registered", version, kind)
	}

	// Register the version
	vr.resources[kind][version] = info

	// Set as default if it's marked as default or no default exists
	if info.Metadata.IsDefault || vr.defaults[kind] == "" {
		vr.defaults[kind] = version
	}

	return nil
}

// GetVersion retrieves type information for a specific resource version
func (vr *VersionRegistry) GetVersion(kind, version string) (ResourceTypeInfo, bool) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	kindVersions, kindExists := vr.resources[kind]
	if !kindExists {
		return ResourceTypeInfo{}, false
	}

	info, versionExists := kindVersions[version]
	return info, versionExists
}

// GetDefaultVersion returns the default version for a resource kind
func (vr *VersionRegistry) GetDefaultVersion(kind string) string {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	return vr.defaults[kind]
}

// SetDefaultVersion sets the default version for a resource kind
func (vr *VersionRegistry) SetDefaultVersion(kind, version string) error {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	// Verify the version exists
	if kindVersions, exists := vr.resources[kind]; exists {
		if _, versionExists := kindVersions[version]; !versionExists {
			return fmt.Errorf("version %s not registered for kind %s", version, kind)
		}
	} else {
		return fmt.Errorf("kind %s not registered", kind)
	}

	vr.defaults[kind] = version
	return nil
}

// ListVersions returns all registered versions for a resource kind
func (vr *VersionRegistry) ListVersions(kind string) []string {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	kindVersions, exists := vr.resources[kind]
	if !exists {
		return nil
	}

	versions := make([]string, 0, len(kindVersions))
	for version := range kindVersions {
		versions = append(versions, version)
	}

	// Sort versions for consistent ordering
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) < 0
	})

	return versions
}

// ListKinds returns all registered resource kinds
func (vr *VersionRegistry) ListKinds() []string {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	kinds := make([]string, 0, len(vr.resources))
	for kind := range vr.resources {
		kinds = append(kinds, kind)
	}

	sort.Strings(kinds)
	return kinds
}

// GetVersionInfo returns detailed information about all versions of a kind
func (vr *VersionRegistry) GetVersionInfo(kind string) map[string]SchemaVersion {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	kindVersions, exists := vr.resources[kind]
	if !exists {
		return nil
	}

	info := make(map[string]SchemaVersion)
	for version, typeInfo := range kindVersions {
		info[version] = typeInfo.Metadata
	}

	return info
}

// CanConvert checks if conversion is possible between two versions
func (vr *VersionRegistry) CanConvert(kind, fromVersion, toVersion string) bool {
	fromInfo, fromExists := vr.GetVersion(kind, fromVersion)
	if !fromExists || fromInfo.Converter == nil {
		return false
	}

	return fromInfo.Converter.CanConvert(fromVersion, toVersion)
}

// Convert performs version conversion using the registered converter
func (vr *VersionRegistry) Convert(kind string, resource interface{}, fromVersion, toVersion string) (interface{}, error) {
	fromInfo, fromExists := vr.GetVersion(kind, fromVersion)
	if !fromExists {
		return nil, fmt.Errorf("source version %s not registered for kind %s", fromVersion, kind)
	}

	if fromInfo.Converter == nil {
		return nil, fmt.Errorf("no converter available for kind %s version %s", kind, fromVersion)
	}

	if !fromInfo.Converter.CanConvert(fromVersion, toVersion) {
		return nil, fmt.Errorf("conversion not supported: %s -> %s for kind %s", fromVersion, toVersion, kind)
	}

	return fromInfo.Converter.Convert(resource, fromVersion, toVersion)
}

// GetStabilityLevel returns the stability level of a version
func GetStabilityLevel(version string) string {
	if strings.Contains(version, "alpha") {
		return "alpha"
	}
	if strings.Contains(version, "beta") {
		return "beta"
	}
	return "stable"
}

// isValidVersion validates version format (v1, v2beta1, v3alpha1, etc.)
func isValidVersion(version string) bool {
	if !strings.HasPrefix(version, "v") {
		return false
	}

	// Use regex for proper validation
	versionRegex := regexp.MustCompile(`^v[0-9]+(?:alpha[0-9]+|beta[0-9]+)?$`)
	return versionRegex.MatchString(version)
}

// compareVersions compares two version strings for sorting
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareVersions(a, b string) int {
	// Simple lexicographic comparison for now
	// Could be enhanced with proper semantic version comparison
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Global version registry instance
var GlobalVersionRegistry = NewVersionRegistry()

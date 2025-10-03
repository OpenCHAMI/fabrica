// Package versioning provides HTTP middleware for API version negotiation.
//
// This middleware implements version negotiation strategy for REST APIs,
// supporting both API group versions (in URLs) and resource schema versions
// (via Accept headers).
package versioning

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// VersionContext contains version information for the current request
type VersionContext struct {
	// RequestedVersion is the version requested by the client via Accept header
	RequestedVersion string

	// DefaultVersion is the default version for the resource type
	DefaultVersion string

	// ServeVersion is the final version that will be served
	ServeVersion string

	// GroupVersion is the API group version from the URL path
	GroupVersion string

	// ResourceKind is the resource type being accessed
	ResourceKind string
}

// VersionContextKey is the context key for version information
type VersionContextKey string

const (
	VersionContextKeyName VersionContextKey = "version_context"
)

// ResourceMapper defines the interface for mapping plural resource names to Kind names.
// Implement this interface to provide custom resource name mappings for your domain.
type ResourceMapper interface {
	// MapResourceToKind converts a plural resource name to a singular Kind name.
	// For example: "devices" -> "Device", "sensors" -> "Sensor"
	MapResourceToKind(pluralName string) string
}

// DefaultResourceMapper provides a simple heuristic-based resource mapper
type DefaultResourceMapper struct{}

// MapResourceToKind provides a simple pluralization heuristic
func (m *DefaultResourceMapper) MapResourceToKind(pluralName string) string {
	// Simple heuristic: remove 's' suffix and capitalize
	if strings.HasSuffix(pluralName, "s") && len(pluralName) > 1 {
		singular := pluralName[:len(pluralName)-1]
		return strings.Title(singular)
	}
	return strings.Title(pluralName)
}

// VersionNegotiationMiddleware provides HTTP middleware for version negotiation
func VersionNegotiationMiddleware(registry *VersionRegistry, mapper ResourceMapper) func(http.Handler) http.Handler {
	if mapper == nil {
		mapper = &DefaultResourceMapper{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := &VersionContext{}

			// Extract API group version from URL path
			ctx.GroupVersion = extractGroupVersionFromPath(r.URL.Path)

			// Extract resource kind from URL path
			pluralName := extractResourceNameFromPath(r.URL.Path)
			if pluralName != "" {
				ctx.ResourceKind = mapper.MapResourceToKind(pluralName)
			}

			// Parse requested version from Accept header
			if acceptHeader := r.Header.Get("Accept"); acceptHeader != "" {
				ctx.RequestedVersion = parseVersionFromAcceptHeader(acceptHeader)
			}

			// Get default version for this resource kind
			if ctx.ResourceKind != "" {
				ctx.DefaultVersion = registry.GetDefaultVersion(ctx.ResourceKind)
			}

			// Negotiate the final version to serve
			ctx.ServeVersion = negotiateVersion(ctx, registry)

			// If version negotiation failed (client requested unsupported version), return 406
			if ctx.ServeVersion == "" && ctx.RequestedVersion != "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotAcceptable)
				errorMsg := fmt.Sprintf(`{"error":"Unsupported version","requested":"%s","supported":%v}`,
					ctx.RequestedVersion,
					registry.ListVersions(ctx.ResourceKind))
				_, _ = w.Write([]byte(errorMsg))
				return
			}

			// Set response Content-Type header with version
			if ctx.ServeVersion != "" {
				contentType := fmt.Sprintf("application/json;version=%s", ctx.ServeVersion)
				w.Header().Set("Content-Type", contentType)
			} else {
				w.Header().Set("Content-Type", "application/json")
			}

			// Add version context to request
			ctxWithVersion := context.WithValue(r.Context(), VersionContextKeyName, ctx)
			next.ServeHTTP(w, r.WithContext(ctxWithVersion))
		})
	}
}

// GetVersionContext extracts version context from the HTTP request context
func GetVersionContext(ctx context.Context) *VersionContext {
	if versionCtx, ok := ctx.Value(VersionContextKeyName).(*VersionContext); ok {
		return versionCtx
	}

	// Return default context if none found
	return &VersionContext{
		GroupVersion: "v1",
		ServeVersion: "v1",
	}
}

// extractGroupVersionFromPath extracts the API group version from URL path
// Examples:
//
//	/apis/inventory/v2/devices -> "v2"
//	/apis/inventory/v1beta1/sensors -> "v1beta1"
//	/devices -> "v1" (fallback)
func extractGroupVersionFromPath(path string) string {
	// Pattern: /apis/{group}/{version}/{resource}
	groupVersionRegex := regexp.MustCompile(`^/apis/[^/]+/([^/]+)/`)
	matches := groupVersionRegex.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}

	// Legacy pattern without /apis prefix
	legacyVersionRegex := regexp.MustCompile(`^/v([0-9]+(?:beta[0-9]+|alpha[0-9]+)?)/`)
	matches = legacyVersionRegex.FindStringSubmatch(path)
	if len(matches) > 1 {
		return "v" + matches[1]
	}

	// Default to v1 if no version found in path
	return "v1"
}

// extractResourceNameFromPath extracts the plural resource name from URL path
// Examples:
//
//	/apis/inventory/v2/devices -> "devices"
//	/apis/inventory/v1/sensors -> "sensors"
//	/devices -> "devices"
func extractResourceNameFromPath(path string) string {
	// Pattern: /apis/{group}/{version}/{resource}
	apiResourceRegex := regexp.MustCompile(`^/apis/[^/]+/[^/]+/([^/]+)`)
	matches := apiResourceRegex.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}

	// Legacy pattern or direct resource access
	legacyResourceRegex := regexp.MustCompile(`^(?:/v[^/]+)?/([^/]+)`)
	matches = legacyResourceRegex.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// parseVersionFromAcceptHeader parses version from Accept header
// Examples:
//
//	"application/json;version=v2beta1" -> "v2beta1"
//	"application/vnd.resource+json;v=v1alpha1" -> "v1alpha1"
//	"application/json" -> ""
func parseVersionFromAcceptHeader(acceptHeader string) string {
	// Standard format: application/json;version=v2beta1
	versionRegex := regexp.MustCompile(`version=([^;,\s]+)`)
	matches := versionRegex.FindStringSubmatch(acceptHeader)
	if len(matches) > 1 {
		return matches[1]
	}

	// Alternative format: application/json;v=v2beta1
	altVersionRegex := regexp.MustCompile(`v=([^;,\s]+)`)
	matches = altVersionRegex.FindStringSubmatch(acceptHeader)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// negotiateVersion determines the final version to serve based on client preferences and availability
func negotiateVersion(ctx *VersionContext, registry *VersionRegistry) string {
	// If no resource kind identified, return group version
	if ctx.ResourceKind == "" {
		return ctx.GroupVersion
	}

	// Get available versions for this resource
	availableVersions := registry.ListVersions(ctx.ResourceKind)
	if len(availableVersions) == 0 {
		// No versions registered, fallback to group version
		return ctx.GroupVersion
	}

	// If client requested a specific version, check if it's available
	if ctx.RequestedVersion != "" {
		for _, version := range availableVersions {
			if version == ctx.RequestedVersion {
				return version
			}
		}

		// Requested version not available - return empty string to signal error
		// The handler should check for this and return 406 Not Acceptable
		return ""
	}

	// Use default version for this resource
	if ctx.DefaultVersion != "" {
		return ctx.DefaultVersion
	}

	// Final fallback to the first available version
	if len(availableVersions) > 0 {
		return availableVersions[0]
	}

	// Ultimate fallback to group version
	return ctx.GroupVersion
}

// ValidateVersion checks if a version string follows the expected format
func ValidateVersion(version string) error {
	if !strings.HasPrefix(version, "v") {
		return fmt.Errorf("version must start with 'v': %s", version)
	}

	versionRegex := regexp.MustCompile(`^v[0-9]+(?:alpha[0-9]+|beta[0-9]+)?$`)
	if !versionRegex.MatchString(version) {
		return fmt.Errorf("invalid version format: %s (expected v1, v2beta1, v3alpha1, etc.)", version)
	}

	return nil
}

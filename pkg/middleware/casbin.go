// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package middleware provides HTTP middleware for Fabrica applications.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
)

// CasbinMiddleware creates a Casbin authorization middleware that integrates with TokenSmith JWT middleware.
// It expects that a JWT middleware has already run and populated the request context with claims.
//
// The middleware will:
// 1. Extract user information from JWT claims in the context
// 2. Determine the resource and action from the HTTP request
// 3. Use Casbin to check if the user is authorized
// 4. Allow or deny the request based on the policy
func CasbinMiddleware(enforcer *casbin.Enforcer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user information from JWT claims
			subject, err := extractSubjectFromContext(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
				return
			}

			// Extract resource and action from request
			resource := extractResource(r)
			action := extractAction(r)

			// Check authorization using Casbin
			allowed, err := enforcer.Enforce(subject, resource, action)
			if err != nil {
				http.Error(w, "Authorization error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			// User is authorized, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// extractSubjectFromContext extracts the user subject from the JWT claims in the request context.
// This function expects that TokenSmith middleware has already validated the JWT and added claims to context.
func extractSubjectFromContext(ctx context.Context) (string, error) {
	// Try to get claims from context using the standard TokenSmith key
	claims := ctx.Value("claims")
	if claims == nil {
		return "", fmt.Errorf("no claims found in context")
	}

	// Handle different claim types that TokenSmith might use
	switch c := claims.(type) {
	case map[string]interface{}:
		// Raw claims map
		if sub, ok := c["sub"].(string); ok && sub != "" {
			return sub, nil
		}
		if sub, ok := c["subject"].(string); ok && sub != "" {
			return sub, nil
		}
		if email, ok := c["email"].(string); ok && email != "" {
			return email, nil
		}
	case interface{ GetSubject() string }:
		// Structured claims with Subject method
		if sub := c.GetSubject(); sub != "" {
			return sub, nil
		}
	}

	return "", fmt.Errorf("no valid subject found in claims")
}

// extractResource determines the resource being accessed from the HTTP request.
// This implementation uses the URL path to determine the resource.
func extractResource(r *http.Request) string {
	path := r.URL.Path

	// Remove leading/trailing slashes and normalize
	path = strings.Trim(path, "/")

	// For REST APIs, the resource is typically the first part of the path
	// e.g., "/api/v1/users/123" -> "users"
	parts := strings.Split(path, "/")

	// Skip common prefixes like "api", "v1", etc.
	resourceIndex := 0
	for i, part := range parts {
		if part != "api" && !strings.HasPrefix(part, "v") {
			resourceIndex = i
			break
		}
	}

	if resourceIndex < len(parts) && parts[resourceIndex] != "" {
		return parts[resourceIndex]
	}

	// Fallback to full path if we can't extract a clean resource name
	return path
}

// extractAction determines the action being performed from the HTTP request.
// This maps HTTP methods to CRUD actions.
func extractAction(r *http.Request) string {
	switch strings.ToUpper(r.Method) {
	case "GET":
		return "read"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(r.Method)
	}
}

// ResourceBasedMiddleware creates a Casbin middleware that uses a specific resource name
// instead of extracting it from the URL path. This is useful for protecting specific resources.
func ResourceBasedMiddleware(enforcer *casbin.Enforcer, resource string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, err := extractSubjectFromContext(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
				return
			}

			action := extractAction(r)

			allowed, err := enforcer.Enforce(subject, resource, action)
			if err != nil {
				http.Error(w, "Authorization error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				http.Error(w, fmt.Sprintf("Forbidden: insufficient permissions for %s", resource), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GroupBasedMiddleware creates a middleware that checks if the user belongs to required groups.
// This provides an alternative to resource-based authorization for role-based access control.
func GroupBasedMiddleware(requiredGroups []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := r.Context().Value("claims")
			if claims == nil {
				http.Error(w, "Unauthorized: no claims", http.StatusUnauthorized)
				return
			}

			var userGroups []string
			switch c := claims.(type) {
			case map[string]interface{}:
				if groups, ok := c["groups"].([]interface{}); ok {
					for _, g := range groups {
						if groupStr, ok := g.(string); ok {
							userGroups = append(userGroups, groupStr)
						}
					}
				}
			}

			// Check if user has any of the required groups
			hasRequiredGroup := false
			for _, required := range requiredGroups {
				for _, userGroup := range userGroups {
					if userGroup == required {
						hasRequiredGroup = true
						break
					}
				}
				if hasRequiredGroup {
					break
				}
			}

			if !hasRequiredGroup {
				http.Error(w, "Forbidden: insufficient group membership", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

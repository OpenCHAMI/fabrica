// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package policy

import (
	"context"
	"net/http"
)

// PermissivePolicy is a policy that allows all operations without authentication.
// This is intended for development and testing purposes only.
//
// WARNING: DO NOT USE IN PRODUCTION
// This policy bypasses all authentication and authorization checks.
//
// Usage:
//
//	policyRegistry.RegisterPolicy("User", policy.NewPermissivePolicy())
//	policyRegistry.RegisterPolicy("Product", policy.NewPermissivePolicy())
type PermissivePolicy struct{}

// NewPermissivePolicy creates a new permissive policy that allows all operations
func NewPermissivePolicy() ResourcePolicy {
	return &PermissivePolicy{}
}

// CanList implements ResourcePolicy.CanList and allows all list operations
func (p *PermissivePolicy) CanList(_ context.Context, _ *AuthContext, _ *http.Request) PolicyDecision {
	return Allow()
}

// CanGet implements ResourcePolicy.CanGet and allows all get operations
func (p *PermissivePolicy) CanGet(_ context.Context, _ *AuthContext, _ *http.Request, _ string) PolicyDecision {
	return Allow()
}

// CanCreate implements ResourcePolicy.CanCreate and allows all create operations
func (p *PermissivePolicy) CanCreate(_ context.Context, _ *AuthContext, _ *http.Request, _ interface{}) PolicyDecision {
	return Allow()
}

// CanUpdate implements ResourcePolicy.CanUpdate and allows all update operations
func (p *PermissivePolicy) CanUpdate(_ context.Context, _ *AuthContext, _ *http.Request, _ string, _ interface{}) PolicyDecision {
	return Allow()
}

// CanDelete implements ResourcePolicy.CanDelete and allows all delete operations
func (p *PermissivePolicy) CanDelete(_ context.Context, _ *AuthContext, _ *http.Request, _ string) PolicyDecision {
	return Allow()
}

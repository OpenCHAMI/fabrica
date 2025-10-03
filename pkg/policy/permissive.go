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

func (p *PermissivePolicy) CanList(ctx context.Context, auth *AuthContext, req *http.Request) PolicyDecision {
	return Allow()
}

func (p *PermissivePolicy) CanGet(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string) PolicyDecision {
	return Allow()
}

func (p *PermissivePolicy) CanCreate(ctx context.Context, auth *AuthContext, req *http.Request, resource interface{}) PolicyDecision {
	return Allow()
}

func (p *PermissivePolicy) CanUpdate(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string, resource interface{}) PolicyDecision {
	return Allow()
}

func (p *PermissivePolicy) CanDelete(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string) PolicyDecision {
	return Allow()
}

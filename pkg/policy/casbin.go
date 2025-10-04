// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package policy

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/persist"
)

// CasbinPolicy wraps a Casbin enforcer to implement the ResourcePolicy interface.
// This allows declarative policy management using Casbin's model and policy files
// instead of writing custom policy code.
//
// Example usage:
//
//	enforcer, _ := casbin.NewEnforcer("model.conf", "policy.csv")
//	policy := policy.NewCasbinPolicy(enforcer)
//	registry.RegisterPolicy("Device", policy)
type CasbinPolicy struct {
	enforcer *casbin.Enforcer
	// resourceExtractor is an optional function to extract resource type from request
	// If nil, uses default path-based extraction
	resourceExtractor func(*http.Request) string
}

// NewCasbinPolicy creates a new Casbin-based policy from an enforcer
func NewCasbinPolicy(enforcer *casbin.Enforcer) ResourcePolicy {
	return &CasbinPolicy{
		enforcer:          enforcer,
		resourceExtractor: nil,
	}
}

// NewCasbinPolicyFromFiles creates a Casbin policy from model and policy files
func NewCasbinPolicyFromFiles(modelPath, policyPath string) (ResourcePolicy, error) {
	enforcer, err := casbin.NewEnforcer(modelPath, policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}
	return NewCasbinPolicy(enforcer), nil
}

// NewCasbinPolicyFromAdapter creates a Casbin policy with a custom adapter (e.g., database)
func NewCasbinPolicyFromAdapter(modelPath string, adapter persist.Adapter) (ResourcePolicy, error) {
	enforcer, err := casbin.NewEnforcer(modelPath, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	// Load policies from adapter
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load policies: %w", err)
	}

	return NewCasbinPolicy(enforcer), nil
}

// SetResourceExtractor sets a custom function to extract resource type from requests
func (p *CasbinPolicy) SetResourceExtractor(fn func(*http.Request) string) {
	p.resourceExtractor = fn
}

// GetEnforcer returns the underlying Casbin enforcer for advanced usage
func (p *CasbinPolicy) GetEnforcer() *casbin.Enforcer {
	return p.enforcer
}

// EnableAutoSave enables automatic policy persistence (requires database adapter)
func (p *CasbinPolicy) EnableAutoSave(enabled bool) {
	p.enforcer.EnableAutoSave(enabled)
}

// CanList implements ResourcePolicy.CanList
func (p *CasbinPolicy) CanList(ctx context.Context, auth *AuthContext, req *http.Request) PolicyDecision {
	if auth == nil {
		return Deny("no authentication context provided")
	}

	resourceType := p.getResourceType(req)
	allowed, err := p.enforcer.Enforce(auth.UserID, resourceType, "list")
	if err != nil {
		return Deny(fmt.Sprintf("policy evaluation error: %v", err))
	}

	if !allowed {
		return Deny(fmt.Sprintf("user %s does not have permission to list %s resources", auth.UserID, resourceType))
	}

	return Allow()
}

// CanGet implements ResourcePolicy.CanGet
func (p *CasbinPolicy) CanGet(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string) PolicyDecision {
	if auth == nil {
		return Deny("no authentication context provided")
	}

	resourceType := p.getResourceType(req)
	allowed, err := p.enforcer.Enforce(auth.UserID, resourceType, "get")
	if err != nil {
		return Deny(fmt.Sprintf("policy evaluation error: %v", err))
	}

	if !allowed {
		return Deny(fmt.Sprintf("user %s does not have permission to get %s resources", auth.UserID, resourceType))
	}

	return Allow()
}

// CanCreate implements ResourcePolicy.CanCreate
func (p *CasbinPolicy) CanCreate(ctx context.Context, auth *AuthContext, req *http.Request, resource interface{}) PolicyDecision {
	if auth == nil {
		return Deny("no authentication context provided")
	}

	resourceType := p.getResourceType(req)
	allowed, err := p.enforcer.Enforce(auth.UserID, resourceType, "create")
	if err != nil {
		return Deny(fmt.Sprintf("policy evaluation error: %v", err))
	}

	if !allowed {
		return Deny(fmt.Sprintf("user %s does not have permission to create %s resources", auth.UserID, resourceType))
	}

	return Allow()
}

// CanUpdate implements ResourcePolicy.CanUpdate
func (p *CasbinPolicy) CanUpdate(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string, resource interface{}) PolicyDecision {
	if auth == nil {
		return Deny("no authentication context provided")
	}

	resourceType := p.getResourceType(req)
	allowed, err := p.enforcer.Enforce(auth.UserID, resourceType, "update")
	if err != nil {
		return Deny(fmt.Sprintf("policy evaluation error: %v", err))
	}

	if !allowed {
		return Deny(fmt.Sprintf("user %s does not have permission to update %s resources", auth.UserID, resourceType))
	}

	return Allow()
}

// CanDelete implements ResourcePolicy.CanDelete
func (p *CasbinPolicy) CanDelete(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string) PolicyDecision {
	if auth == nil {
		return Deny("no authentication context provided")
	}

	resourceType := p.getResourceType(req)
	allowed, err := p.enforcer.Enforce(auth.UserID, resourceType, "delete")
	if err != nil {
		return Deny(fmt.Sprintf("policy evaluation error: %v", err))
	}

	if !allowed {
		return Deny(fmt.Sprintf("user %s does not have permission to delete %s resources", auth.UserID, resourceType))
	}

	return Allow()
}

// getResourceType extracts the resource type from the request
// Uses custom extractor if set, otherwise falls back to path parsing
func (p *CasbinPolicy) getResourceType(req *http.Request) string {
	if p.resourceExtractor != nil {
		return p.resourceExtractor(req)
	}

	// Default: extract from path (e.g., "/api/v1/devices" -> "Device")
	// This assumes standard Fabrica routing: /api/v1/{resources}
	path := req.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Find the resource segment (after "api/v1")
	for i, part := range parts {
		if part == "v1" && i+1 < len(parts) {
			resource := parts[i+1]
			// Singularize and capitalize (simple heuristic)
			resource = strings.TrimSuffix(resource, "s")
			if len(resource) > 0 {
				resource = strings.ToUpper(resource[:1]) + resource[1:]
			}
			return resource
		}
	}

	return "Unknown"
}

// Policy Management Methods
// These methods allow runtime policy modifications

// AddPolicy adds a policy rule
func (p *CasbinPolicy) AddPolicy(subject, object, action string) (bool, error) {
	return p.enforcer.AddPolicy(subject, object, action)
}

// RemovePolicy removes a policy rule
func (p *CasbinPolicy) RemovePolicy(subject, object, action string) (bool, error) {
	return p.enforcer.RemovePolicy(subject, object, action)
}

// AddRoleForUser assigns a role to a user
func (p *CasbinPolicy) AddRoleForUser(user, role string) (bool, error) {
	return p.enforcer.AddRoleForUser(user, role)
}

// DeleteRoleForUser removes a role from a user
func (p *CasbinPolicy) DeleteRoleForUser(user, role string) (bool, error) {
	return p.enforcer.DeleteRoleForUser(user, role)
}

// GetRolesForUser returns all roles for a user
func (p *CasbinPolicy) GetRolesForUser(user string) ([]string, error) {
	return p.enforcer.GetRolesForUser(user)
}

// GetUsersForRole returns all users that have a role
func (p *CasbinPolicy) GetUsersForRole(role string) ([]string, error) {
	return p.enforcer.GetUsersForRole(role)
}

// GetPolicy returns all policy rules
func (p *CasbinPolicy) GetPolicy() ([][]string, error) {
	return p.enforcer.GetPolicy()
}

// SavePolicy saves the current policy to storage (requires adapter with save support)
func (p *CasbinPolicy) SavePolicy() error {
	return p.enforcer.SavePolicy()
}

// LoadPolicy reloads policies from storage
func (p *CasbinPolicy) LoadPolicy() error {
	return p.enforcer.LoadPolicy()
}

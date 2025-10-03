package policy

import (
	"context"
	"net/http"
)

// AuthContext represents the authentication context from JWT claims
// This should match your authentication middleware's AuthContext
type AuthContext struct {
	// Standard JWT claims
	UserID   string   `json:"sub,omitempty"`
	Email    string   `json:"email,omitempty"`
	Username string   `json:"preferred_username,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	Groups   []string `json:"groups,omitempty"`

	// Arbitrary claims from JWT - developers can access any field
	Claims map[string]interface{} `json:"-"`
}

// PolicyDecision represents the result of an authorization decision
type PolicyDecision struct {
	Allowed bool
	Reason  string
}

// Allow creates an allowed policy decision
func Allow() PolicyDecision {
	return PolicyDecision{Allowed: true}
}

// Deny creates a denied policy decision with a reason
func Deny(reason string) PolicyDecision {
	return PolicyDecision{Allowed: false, Reason: reason}
}

// ResourcePolicy defines the basic interface for resource authorization
type ResourcePolicy interface {
	CanList(ctx context.Context, auth *AuthContext, req *http.Request) PolicyDecision
	CanGet(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string) PolicyDecision
	CanCreate(ctx context.Context, auth *AuthContext, req *http.Request, resource interface{}) PolicyDecision
	CanUpdate(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string, resource interface{}) PolicyDecision
	CanDelete(ctx context.Context, auth *AuthContext, req *http.Request, resourceUID string) PolicyDecision
}

// PolicyRegistry holds all the policy implementations
type PolicyRegistry struct {
	policies map[string]ResourcePolicy
}

// NewPolicyRegistry creates a new policy registry
func NewPolicyRegistry() *PolicyRegistry {
	return &PolicyRegistry{
		policies: make(map[string]ResourcePolicy),
	}
}

// RegisterPolicy registers a policy for a resource type
func (r *PolicyRegistry) RegisterPolicy(resourceName string, policy ResourcePolicy) {
	r.policies[resourceName] = policy
}

// GetPolicy retrieves a policy for a resource type
func (r *PolicyRegistry) GetPolicy(resourceName string) (ResourcePolicy, bool) {
	policy, exists := r.policies[resourceName]
	return policy, exists
}

// Helper functions for common authorization patterns

// HasRole checks if the user has a specific role
func HasRole(auth *AuthContext, role string) bool {
	if auth == nil {
		return false
	}
	for _, r := range auth.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func HasAnyRole(auth *AuthContext, roles ...string) bool {
	if auth == nil {
		return false
	}
	for _, required := range roles {
		if HasRole(auth, required) {
			return true
		}
	}
	return false
}

// GetClaim extracts a claim value from the JWT claims
// This allows access to arbitrary JWT fields like job_id, customer_id, tenant_id, etc.
func GetClaim(auth *AuthContext, key string) (interface{}, bool) {
	if auth == nil || auth.Claims == nil {
		return nil, false
	}
	value, exists := auth.Claims[key]
	return value, exists
}

// GetStringClaim extracts a string claim value
func GetStringClaim(auth *AuthContext, key string) (string, bool) {
	value, exists := GetClaim(auth, key)
	if !exists {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

// GetStringSliceClaim extracts a string slice claim value
func GetStringSliceClaim(auth *AuthContext, key string) ([]string, bool) {
	value, exists := GetClaim(auth, key)
	if !exists {
		return nil, false
	}

	// Handle both []string and []interface{} containing strings
	switch v := value.(type) {
	case []string:
		return v, true
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, len(result) > 0
	default:
		return nil, false
	}
}

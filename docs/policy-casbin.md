<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Casbin Policy Management

This guide covers using [Casbin](https://casbin.org) for authorization in Fabrica-generated APIs.

## Overview

Casbin is a powerful authorization library that supports multiple access control models (RBAC, ABAC, ACL). Fabrica integrates Casbin to provide **declarative policy management** without writing custom authorization code.

### Benefits

- **Declarative policies** - Define policies in CSV files, not Go code
- **Runtime updates** - Add/remove permissions without redeployment
- **Multiple models** - RBAC, ABAC, ACL support
- **Database persistence** - Policies stored alongside resources
- **Policy administration API** - Manage policies via REST endpoints
- **No code needed** - Common authorization patterns work out-of-the-box

## Quick Start

### 1. Generated Files

When you run `fabrica generate`, the following policy files are created:

```
policies/
├── model.conf      # Casbin RBAC model definition
└── policy.csv      # Default policies for your resources
```

### 2. Understanding the Model

The `model.conf` defines **how** authorization works:

```ini
[request_definition]
r = sub, obj, act    # subject (user), object (resource), action (operation)

[policy_definition]
p = sub, obj, act    # policy rules

[role_definition]
g = _, _             # role inheritance

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

### 3. Understanding Policies

The `policy.csv` defines **who** can do **what**:

```csv
# Format: p, subject, object, action

# Admin role - full access
p, admin, *, *

# Device manager - full access to devices
p, device-manager, Device, list
p, device-manager, Device, get
p, device-manager, Device, create
p, device-manager, Device, update
p, device-manager, Device, delete

# Device viewer - read-only
p, device-viewer, Device, list
p, device-viewer, Device, get

# Role assignments
g, alice@example.com, admin
g, bob@example.com, device-manager
g, carol@example.com, device-viewer
```

## Using Casbin in Your API

### Option 1: File-Based Policies (Simple)

```go
package main

import (
    "log"
    "github.com/OpenCHAMI/fabrica/pkg/policy"
)

func main() {
    // Create Casbin policy from files
    casbinPolicy, err := policy.NewCasbinPolicyFromFiles(
        "policies/model.conf",
        "policies/policy.csv",
    )
    if err != nil {
        log.Fatal(err)
    }

    // Register for all resources
    registry := policy.NewPolicyRegistry()
    registry.RegisterPolicy("Device", casbinPolicy)
    registry.RegisterPolicy("User", casbinPolicy)

    // Use in handlers (already integrated)
}
```

### Option 2: Database-Backed Policies (Production)

```go
package main

import (
    "log"
    "github.com/OpenCHAMI/fabrica/pkg/policy"
    "entgo.io/ent/dialect/sql"
)

func main() {
    // Create database connection
    drv, _ := sql.Open("postgres", "postgresql://...")
    client := ent.NewClient(ent.Driver(drv))

    // Create Casbin policy with Ent adapter
    casbinPolicy, err := policy.NewCasbinPolicyWithEntAdapter(
        client,
        "policies/model.conf",
    )
    if err != nil {
        log.Fatal(err)
    }

    // Enable auto-save for immediate persistence
    if cp, ok := casbinPolicy.(*policy.CasbinPolicy); ok {
        cp.EnableAutoSave(true)
    }

    // Register policies
    registry := policy.NewPolicyRegistry()
    registry.RegisterPolicy("Device", casbinPolicy)

    // Policies are now stored in database and survive restarts
}
```

## Policy Administration API

Fabrica can generate policy management endpoints. Add these routes to your server:

```go
// In your routes setup
policyHandlers := handlers.NewPolicyHandlers(casbinPolicy.(*policy.CasbinPolicy))

http.HandleFunc("/api/v1/policies", policyHandlers.GetPolicies)
http.HandleFunc("/api/v1/policies/add", policyHandlers.AddPolicy)
http.HandleFunc("/api/v1/policies/remove", policyHandlers.RemovePolicy)
http.HandleFunc("/api/v1/roles", policyHandlers.AddRoleForUser)
http.HandleFunc("/api/v1/roles/remove", policyHandlers.RemoveRoleFromUser)
http.HandleFunc("/api/v1/users/roles", policyHandlers.GetRolesForUser)
```

### API Examples

**Get all policies:**
```bash
curl http://localhost:8080/api/v1/policies
```

**Add a policy:**
```bash
curl -X POST http://localhost:8080/api/v1/policies/add \
  -H "Content-Type: application/json" \
  -d '{
    "subject": "device-manager",
    "object": "Device",
    "action": "delete"
  }'
```

**Assign a role to a user:**
```bash
curl -X POST http://localhost:8080/api/v1/roles \
  -H "Content-Type: application/json" \
  -d '{
    "user": "alice@example.com",
    "role": "admin"
  }'
```

**Get user's roles:**
```bash
curl http://localhost:8080/api/v1/users/roles?user=alice@example.com
```

## Common Patterns

### Pattern 1: Resource-Specific Roles

Each resource gets three standard roles:

```csv
# Device roles
p, device-manager, Device, *
p, device-editor, Device, create
p, device-editor, Device, update
p, device-viewer, Device, list
p, device-viewer, Device, get

# User roles
p, user-manager, User, *
p, user-viewer, User, list
p, user-viewer, User, get
```

### Pattern 2: Global Admin

```csv
# Admin can do everything
p, admin, *, *

# Assign admin role
g, superuser@example.com, admin
```

### Pattern 3: Organization-Based Access

For multi-tenancy, extend the model (see Advanced section).

## Default Roles

Fabrica generates these roles for each resource:

| Role | Permissions | Use Case |
|------|-------------|----------|
| `{resource}-manager` | Full CRUD | Team leads, resource owners |
| `{resource}-editor` | Create, Read, Update | Regular users |
| `{resource}-viewer` | Read-only | Read-only access, auditors |
| `admin` | All resources | System administrators |

## Migration from Custom Policies

If you have existing custom policies:

1. **Keep both** - Use Casbin for simple RBAC, custom policies for complex logic
2. **Gradual migration** - Move simple role checks to Casbin first
3. **Eventually consolidate** - Move complex logic to ABAC models

Example migration:

```go
// Before: Custom policy
type DevicePolicy struct{}

func (p *DevicePolicy) CanDelete(ctx context.Context, auth *policy.AuthContext, req *http.Request, uid string) policy.PolicyDecision {
    if policy.HasRole(auth, "admin") || policy.HasRole(auth, "device-manager") {
        return policy.Allow()
    }
    return policy.Deny("insufficient permissions")
}

// After: Casbin policy (no code needed!)
// Just add to policy.csv:
// p, admin, Device, delete
// p, device-manager, Device, delete
```

## Advanced: ABAC (Attribute-Based Access Control)

For resource-level permissions (e.g., "users can only delete their own devices"):

**Create an ABAC model** (`policies/abac_model.conf`):

```ini
[request_definition]
r = sub, obj, act, res

[policy_definition]
p = sub, obj, act, res

[matchers]
m = r.sub == r.res.owner || r.sub.role == "admin"
```

**Use in code:**

```go
// Check if user owns the resource
resourceAttrs := map[string]interface{}{
    "owner": device.Metadata.Annotations["owner"],
}

userAttrs := map[string]interface{}{
    "id": auth.UserID,
    "role": auth.Roles[0],
}

allowed := enforcer.Enforce(userAttrs, "Device", "delete", resourceAttrs)
```

## Troubleshooting

### Policy not taking effect

```bash
# Reload policies from database
enforcer.LoadPolicy()

# Check current policies
enforcer.GetPolicy()

# Enable debug logging
enforcer.EnableLog(true)
```

### User has role but still denied

Check role inheritance:

```bash
# Verify role assignment
enforcer.GetRolesForUser("user@example.com")

# Check if policy exists
enforcer.GetPolicy()
```

### Database policies not persisting

```bash
# Enable auto-save
enforcer.EnableAutoSave(true)

# Manually save
enforcer.SavePolicy()
```

## Best Practices

1. **Start simple** - Use RBAC before moving to ABAC
2. **Use database adapter** - For production deployments
3. **Version control policies** - Keep `policy.csv` in git
4. **Test policies** - Write tests for authorization logic
5. **Audit policy changes** - Log all policy modifications
6. **Separate admin policies** - Don't mix admin and user policies
7. **Use meaningful role names** - `device-manager` not `role1`

## Security Considerations

- **Never trust client input** - Always verify JWT claims server-side
- **Use HTTPS** - Protect JWT tokens in transit
- **Rotate secrets** - Change JWT signing keys regularly
- **Audit logs** - Track who changed what policies
- **Least privilege** - Grant minimal permissions needed
- **Review regularly** - Periodically audit role assignments

## Resources

- [Casbin Documentation](https://casbin.org/docs/overview)
- [Casbin RBAC Guide](https://casbin.org/docs/rbac)
- [Casbin ABAC Guide](https://casbin.org/docs/abac)
- [Online Editor](https://casbin.org/editor) - Test policies interactively
- [Fabrica Policy Examples](https://github.com/OpenCHAMI/fabrica/tree/main/examples)

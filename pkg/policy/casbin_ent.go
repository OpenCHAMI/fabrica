// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package policy

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	entadapter "github.com/casbin/ent-adapter"
)

// NewCasbinPolicyWithEntAdapter creates a Casbin policy using Ent for persistence.
// This allows policies to be stored in the database alongside other resources.
//
// Example usage:
//
//	// Create Casbin policy with database adapter
//	policy, err := policy.NewCasbinPolicyWithEntAdapter(
//	    "postgres",
//	    "postgresql://user:pass@localhost/db?sslmode=disable",
//	    "policies/model.conf",
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Register policy
//	registry.RegisterPolicy("Device", policy)
//
// The adapter automatically creates the casbin_rule table and manages policy persistence.
func NewCasbinPolicyWithEntAdapter(driverName, dataSourceName, modelPath string) (ResourcePolicy, error) {
	// Create Ent adapter
	adapter, err := entadapter.NewAdapter(driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create ent adapter: %w", err)
	}

	// Create enforcer with Ent adapter
	enforcer, err := casbin.NewEnforcer(modelPath, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	// Load existing policies from database
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load policies: %w", err)
	}

	// Enable auto-save so policy changes are immediately persisted
	enforcer.EnableAutoSave(true)

	return NewCasbinPolicy(enforcer), nil
}

// InitializeCasbinPolicies loads default policies into Casbin from a CSV file.
// This is useful for bootstrapping policies on first run.
//
// Example usage:
//
//	casbinPolicy := policy.NewCasbinPolicy(enforcer)
//	if err := policy.InitializeCasbinPolicies(casbinPolicy, "policies/policy.csv"); err != nil {
//	    log.Printf("Failed to load default policies: %v", err)
//	}
func InitializeCasbinPolicies(casbinPolicy ResourcePolicy, policyFilePath string) error {
	cp, ok := casbinPolicy.(*CasbinPolicy)
	if !ok {
		return fmt.Errorf("provided policy is not a CasbinPolicy")
	}

	enforcer := cp.GetEnforcer()

	// Load policies from CSV file
	// Note: This adds policies, doesn't replace existing ones
	if err := enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("failed to load policies from file: %w", err)
	}

	return nil
}

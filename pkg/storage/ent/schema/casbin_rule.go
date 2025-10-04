// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package schema provides Ent schemas for database entities including Casbin policy persistence.
//
// NOTE: This file provides the Ent schema for Casbin policy persistence.
// It defines the database structure for storing Casbin policies, allowing
// policies to be persisted and managed through the database.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CasbinRule holds the schema definition for Casbin policy rules.
// This schema is compatible with the standard Casbin adapter format.
type CasbinRule struct {
	ent.Schema
}

// Fields of the CasbinRule.
func (CasbinRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("ptype").
			NotEmpty().
			Comment("Policy type (p for policy, g for role)"),

		field.String("v0").
			Optional().
			Comment("Subject (user or role)"),

		field.String("v1").
			Optional().
			Comment("Object (resource type)"),

		field.String("v2").
			Optional().
			Comment("Action (list, get, create, update, delete)"),

		field.String("v3").
			Optional().
			Comment("Additional parameter"),

		field.String("v4").
			Optional().
			Comment("Additional parameter"),

		field.String("v5").
			Optional().
			Comment("Additional parameter"),
	}
}

// Indexes of the CasbinRule.
func (CasbinRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ptype", "v0", "v1", "v2"),
	}
}

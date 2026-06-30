// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

/*
Package annotations provides parsing and validation for Fabrica code generation directives.

Fabrica uses Go comment annotations (similar to Kubernetes code generators) to configure
how resources are stored in the database. Annotations control table schemas, field-level
transformations (hashing, encryption), indexes, and constraints.

# Overview

The package supports two storage modes:

  - Generic (default): All resources in one table with JSON spec/status columns
  - Dedicated: Per-resource table with flattened field columns

Dedicated storage enables field-level control via annotations:

  - Hashing (bcrypt, argon2, sha256) for passwords and tokens
  - Encryption (AES-128/192/256) for sensitive data
  - Database indexes (btree, gin, gist, hash) for query performance
  - Constraints (unique, immutable, default values)

# Quick Start

Define a resource with annotations:

	// +fabrica:resource
	// +fabrica:storage=dedicated
	type Token struct {
	    metav1.TypeMeta   `json:",inline"`
	    metav1.ObjectMeta `json:"metadata,omitempty"`
	    Spec   TokenSpec   `json:"spec,omitempty"`
	}

	type TokenSpec struct {
	    // +fabrica:field:storage=hashed:bcrypt:cost=12
	    // +fabrica:field:sensitive
	    // +fabrica:field:immutable
	    Value string `json:"value"`

	    // +fabrica:field:index
	    // +fabrica:field:unique
	    Name string `json:"name"`
	}

Parse annotations from Go AST:

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "types.go", src, parser.ParseComments)

	var typeSpec *ast.TypeSpec
	var docComments *ast.CommentGroup
	ast.Inspect(file, func(n ast.Node) bool {
	    if gd, ok := n.(*ast.GenDecl); ok {
	        for _, spec := range gd.Specs {
	            if ts, ok := spec.(*ast.TypeSpec); ok {
	                typeSpec = ts
	                docComments = gd.Doc
	                return false
	            }
	        }
	    }
	    return true
	})

	annots, err := annotations.ParseResourceAnnotations(typeSpec, docComments)
	if err != nil {
	    log.Fatal(err)
	}

	if err := annotations.Validate(annots); err != nil {
	    log.Fatal(err)
	}

Use annotations in code generation:

	if annots.StorageMode == annotations.StorageModeDedicated {
	    for fieldName, fieldAnnots := range annots.Fields {
	        if fieldAnnots.Storage != nil && fieldAnnots.Storage.Type == annotations.StorageTypeHashed {
	            // Generate bcrypt hashing logic
	        }
	    }
	}

# Resource-Level Annotations

	+fabrica:resource         - Mark type as Fabrica resource
	+fabrica:storage=generic  - Use shared resources table (default)
	+fabrica:storage=dedicated - Use dedicated table per resource

# Field-Level Annotations

Storage transformations:

	+fabrica:field:storage=hashed:bcrypt[:cost=12]
	+fabrica:field:storage=hashed:argon2[:memory=65536]
	+fabrica:field:storage=hashed:sha256
	+fabrica:field:storage=encrypted:aes256[:key=vault]

Constraints:

	+fabrica:field:sensitive    - Exclude from logs
	+fabrica:field:immutable    - Prevent updates
	+fabrica:field:unique       - Unique constraint
	+fabrica:field:default=val  - Database default

Indexes:

	+fabrica:field:index        - B-tree index (default)
	+fabrica:field:index=gin    - GIN index (full-text, PostgreSQL)
	+fabrica:field:index=gist   - GiST index (spatial, PostgreSQL)
	+fabrica:field:index=hash   - Hash index

# Validation

The package performs two levels of validation:

1. Semantic validation (Validate): Checks parameter ranges, conflicting annotations

2. Database-specific validation (ValidateForDatabase): Checks feature support per database

Example:

	if err := annotations.Validate(annots); err != nil {
	    return fmt.Errorf("invalid annotations: %w", err)
	}

	if err := annotations.ValidateForDatabase(annots, "sqlite3"); err != nil {
	    return fmt.Errorf("incompatible with SQLite: %w", err)
	}

# Error Types

ParseError - Syntax error in annotation:

	failed to parse annotation "+fabrica:storage=invalid": unknown storage mode

ValidationError - Semantic error:

	invalid annotation "field Value bcrypt cost": bcrypt cost must be 4-31, got 32

# Database Compatibility

Index types by database:

	PostgreSQL: btree, gin, gist, hash (all supported)
	MySQL:      btree, gin, hash (no gist)
	SQLite:     btree only

All hashing/encryption algorithms are supported on all databases (application-level).

# Best Practices

Security:

  - Always mark hashed/encrypted fields as sensitive
  - Use bcrypt cost ≥12 for passwords (14 recommended)
  - Don't hash searchable fields (prevents lookups)

Performance:

  - Index frequently queried fields
  - Use GIN indexes for full-text search
  - Avoid over-indexing (slows writes)

Maintainability:

  - Group related annotations together
  - Document why annotations are needed
  - Validate in CI pipeline

# Integration

The annotations package is designed to integrate with Fabrica's code generator:

1. Generator parses Go source files with go/ast
2. Calls ParseResourceAnnotations for each type
3. Validates annotations
4. Uses annotations to select schema templates
5. Generates dedicated Ent schemas for annotated resources

See pkg/annotations/README.md for complete documentation.
*/
package annotations

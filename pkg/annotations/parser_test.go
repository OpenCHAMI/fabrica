// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestParseAnnotationValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple annotation",
			input:    "+fabrica:resource",
			expected: []string{"resource"},
		},
		{
			name:     "key-value annotation",
			input:    "+fabrica:storage=dedicated",
			expected: []string{"storage=dedicated"},
		},
		{
			name:     "nested annotation",
			input:    "+fabrica:field:storage=hashed:bcrypt:cost=12",
			expected: []string{"field", "storage=hashed", "bcrypt", "cost=12"},
		},
		{
			name:     "with whitespace",
			input:    "  +fabrica:field:sensitive  ",
			expected: []string{"field", "sensitive"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAnnotationValue(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d parts, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, part := range result {
				if part != tt.expected[i] {
					t.Errorf("part %d: expected %q, got %q", i, tt.expected[i], part)
				}
			}
		})
	}
}

func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectKey string
		expectVal string
		hasValue  bool
	}{
		{
			name:      "simple key-value",
			input:     "storage=dedicated",
			expectKey: "storage",
			expectVal: "dedicated",
			hasValue:  true,
		},
		{
			name:      "no value",
			input:     "sensitive",
			expectKey: "sensitive",
			expectVal: "",
			hasValue:  false,
		},
		{
			name:      "numeric value",
			input:     "cost=12",
			expectKey: "cost",
			expectVal: "12",
			hasValue:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, hasValue := ParseKeyValue(tt.input)
			if key != tt.expectKey {
				t.Errorf("expected key %q, got %q", tt.expectKey, key)
			}
			if value != tt.expectVal {
				t.Errorf("expected value %q, got %q", tt.expectVal, value)
			}
			if hasValue != tt.hasValue {
				t.Errorf("expected hasValue %v, got %v", tt.hasValue, hasValue)
			}
		})
	}
}

func TestParseIntValue(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		min       int
		max       int
		expected  int
		expectErr bool
	}{
		{
			name:      "valid value",
			input:     "12",
			min:       4,
			max:       31,
			expected:  12,
			expectErr: false,
		},
		{
			name:      "min boundary",
			input:     "4",
			min:       4,
			max:       31,
			expected:  4,
			expectErr: false,
		},
		{
			name:      "max boundary",
			input:     "31",
			min:       4,
			max:       31,
			expected:  31,
			expectErr: false,
		},
		{
			name:      "below min",
			input:     "3",
			min:       4,
			max:       31,
			expectErr: true,
		},
		{
			name:      "above max",
			input:     "32",
			min:       4,
			max:       31,
			expectErr: true,
		},
		{
			name:      "not a number",
			input:     "abc",
			min:       4,
			max:       31,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseIntValue(tt.input, tt.min, tt.max)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestIsFabricaAnnotation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid annotation",
			input:    "// +fabrica:resource",
			expected: true,
		},
		{
			name:     "no comment prefix",
			input:    "+fabrica:resource",
			expected: true,
		},
		{
			name:     "regular comment",
			input:    "// This is a regular comment",
			expected: false,
		},
		{
			name:     "empty line",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFabricaAnnotation(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseResourceAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		checkResult func(*testing.T, *ResourceAnnotations, error)
	}{
		{
			name: "basic resource with dedicated storage",
			source: `package test

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	Name string
}`,
			checkResult: func(t *testing.T, result *ResourceAnnotations, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !result.IsResource {
					t.Error("expected IsResource=true")
				}
				if result.StorageMode != StorageModeDedicated {
					t.Errorf("expected StorageMode=dedicated, got %s", result.StorageMode)
				}
			},
		},
		{
			name: "field with bcrypt hashing",
			source: `package test

// +fabrica:resource
type Token struct {
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	Value string
}`,
			checkResult: func(t *testing.T, result *ResourceAnnotations, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fieldAnnotations, ok := result.Fields["Value"]
				if !ok {
					t.Fatal("expected field annotation for 'Value'")
				}

				if fieldAnnotations.Storage == nil {
					t.Fatal("expected Storage config")
				}

				if fieldAnnotations.Storage.Type != StorageTypeHashed {
					t.Errorf("expected StorageType=hashed, got %s", fieldAnnotations.Storage.Type)
				}

				if fieldAnnotations.Storage.Hash == nil {
					t.Fatal("expected Hash config")
				}

				if fieldAnnotations.Storage.Hash.Algorithm != HashAlgorithmBcrypt {
					t.Errorf("expected bcrypt algorithm, got %s", fieldAnnotations.Storage.Hash.Algorithm)
				}

				if fieldAnnotations.Storage.Hash.Cost != 12 {
					t.Errorf("expected cost=12, got %d", fieldAnnotations.Storage.Hash.Cost)
				}
			},
		},
		{
			name: "field with multiple annotations",
			source: `package test

type Token struct {
	// +fabrica:field:storage=hashed:bcrypt:cost=10
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Value string
}`,
			checkResult: func(t *testing.T, result *ResourceAnnotations, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fieldAnnotations, ok := result.Fields["Value"]
				if !ok {
					t.Fatal("expected field annotation for 'Value'")
				}

				if !fieldAnnotations.Sensitive {
					t.Error("expected Sensitive=true")
				}

				if !fieldAnnotations.Immutable {
					t.Error("expected Immutable=true")
				}

				if fieldAnnotations.Storage == nil {
					t.Fatal("expected Storage config")
				}

				if fieldAnnotations.Storage.Hash.Cost != 10 {
					t.Errorf("expected cost=10, got %d", fieldAnnotations.Storage.Hash.Cost)
				}
			},
		},
		{
			name: "field with index",
			source: `package test

type Token struct {
	// +fabrica:field:index
	Name string
}`,
			checkResult: func(t *testing.T, result *ResourceAnnotations, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fieldAnnotations, ok := result.Fields["Name"]
				if !ok {
					t.Fatal("expected field annotation for 'Name'")
				}

				if fieldAnnotations.Index == nil {
					t.Fatal("expected Index config")
				}

				if fieldAnnotations.Index.Type != IndexTypeBTree {
					t.Errorf("expected IndexType=btree, got %s", fieldAnnotations.Index.Type)
				}
			},
		},
		{
			name: "field with GIN index",
			source: `package test

type Document struct {
	// +fabrica:field:index=gin
	Content string
}`,
			checkResult: func(t *testing.T, result *ResourceAnnotations, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fieldAnnotations, ok := result.Fields["Content"]
				if !ok {
					t.Fatal("expected field annotation for 'Content'")
				}

				if fieldAnnotations.Index == nil {
					t.Fatal("expected Index config")
				}

				if fieldAnnotations.Index.Type != IndexTypeGIN {
					t.Errorf("expected IndexType=gin, got %s", fieldAnnotations.Index.Type)
				}
			},
		},
		{
			name: "field with default value",
			source: `package test

type Config struct {
	// +fabrica:field:default=true
	Enabled bool
}`,
			checkResult: func(t *testing.T, result *ResourceAnnotations, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fieldAnnotations, ok := result.Fields["Enabled"]
				if !ok {
					t.Fatal("expected field annotation for 'Enabled'")
				}

				if fieldAnnotations.Default != "true" {
					t.Errorf("expected default=true, got %s", fieldAnnotations.Default)
				}
			},
		},
		{
			name: "field with unique constraint",
			source: `package test

type User struct {
	// +fabrica:field:unique
	Email string
}`,
			checkResult: func(t *testing.T, result *ResourceAnnotations, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fieldAnnotations, ok := result.Fields["Email"]
				if !ok {
					t.Fatal("expected field annotation for 'Email'")
				}

				if !fieldAnnotations.Unique {
					t.Error("expected Unique=true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("failed to parse source: %v", err)
			}

			// Find the type declaration and its parent GenDecl
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

			if typeSpec == nil {
				t.Fatal("no type declaration found in source")
			}

			result, err := ParseResourceAnnotations(typeSpec, docComments)
			tt.checkResult(t, result, err)
		})
	}
}

func TestParseHashedStorageDefaultCost(t *testing.T) {
	source := `package test

type Token struct {
	// +fabrica:field:storage=hashed:bcrypt
	Value string
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

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

	result, err := ParseResourceAnnotations(typeSpec, docComments)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fieldAnnotations := result.Fields["Value"]
	if fieldAnnotations.Storage.Hash.Cost != 12 {
		t.Errorf("expected default bcrypt cost=12, got %d", fieldAnnotations.Storage.Hash.Cost)
	}
}

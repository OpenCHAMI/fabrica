// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		annotations *ResourceAnnotations
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid dedicated storage with field annotations",
			annotations: &ResourceAnnotations{
				IsResource:  true,
				StorageMode: StorageModeDedicated,
				Fields: map[string]*FieldAnnotations{
					"Value": {
						FieldName: "Value",
						Storage: &StorageConfig{
							Type: StorageTypeHashed,
							Hash: &HashConfig{
								Algorithm: HashAlgorithmBcrypt,
								Cost:      12,
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "dedicated storage without field annotations",
			annotations: &ResourceAnnotations{
				IsResource:  true,
				StorageMode: StorageModeDedicated,
				Fields:      map[string]*FieldAnnotations{},
			},
			expectError: true,
			errorMsg:    "requires at least one field annotation",
		},
		{
			name: "invalid bcrypt cost (too low)",
			annotations: &ResourceAnnotations{
				IsResource:  true,
				StorageMode: StorageModeDedicated,
				Fields: map[string]*FieldAnnotations{
					"Value": {
						FieldName: "Value",
						Storage: &StorageConfig{
							Type: StorageTypeHashed,
							Hash: &HashConfig{
								Algorithm: HashAlgorithmBcrypt,
								Cost:      3,
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "bcrypt cost must be 4-31",
		},
		{
			name: "invalid bcrypt cost (too high)",
			annotations: &ResourceAnnotations{
				IsResource:  true,
				StorageMode: StorageModeDedicated,
				Fields: map[string]*FieldAnnotations{
					"Value": {
						FieldName: "Value",
						Storage: &StorageConfig{
							Type: StorageTypeHashed,
							Hash: &HashConfig{
								Algorithm: HashAlgorithmBcrypt,
								Cost:      32,
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "bcrypt cost must be 4-31",
		},
		{
			name: "immutable field with default value",
			annotations: &ResourceAnnotations{
				IsResource:  true,
				StorageMode: StorageModeDedicated,
				Fields: map[string]*FieldAnnotations{
					"Status": {
						FieldName: "Status",
						Immutable: true,
						Default:   "pending",
					},
				},
			},
			expectError: true,
			errorMsg:    "immutable fields should not have database defaults",
		},
		{
			name: "valid encryption config",
			annotations: &ResourceAnnotations{
				IsResource:  true,
				StorageMode: StorageModeDedicated,
				Fields: map[string]*FieldAnnotations{
					"Secret": {
						FieldName: "Secret",
						Storage: &StorageConfig{
							Type: StorageTypeEncrypted,
							Encryption: &EncryptionConfig{
								Algorithm: "aes256",
								KeySource: "vault",
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid encryption algorithm",
			annotations: &ResourceAnnotations{
				IsResource:  true,
				StorageMode: StorageModeDedicated,
				Fields: map[string]*FieldAnnotations{
					"Secret": {
						FieldName: "Secret",
						Storage: &StorageConfig{
							Type: StorageTypeEncrypted,
							Encryption: &EncryptionConfig{
								Algorithm: "rot13",
								KeySource: "env",
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "unknown encryption algorithm",
		},
		{
			name: "not a resource (skip validation)",
			annotations: &ResourceAnnotations{
				IsResource:  false,
				StorageMode: StorageModeDedicated,
				Fields:      map[string]*FieldAnnotations{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.annotations)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateForDatabase(t *testing.T) {
	tests := []struct {
		name        string
		annotations *ResourceAnnotations
		dbDriver    string
		expectError bool
		errorMsg    string
	}{
		{
			name: "PostgreSQL supports all index types",
			annotations: &ResourceAnnotations{
				IsResource: true,
				Fields: map[string]*FieldAnnotations{
					"Content": {
						FieldName: "Content",
						Index: &IndexConfig{
							Type: IndexTypeGIN,
						},
					},
				},
			},
			dbDriver:    "postgres",
			expectError: false,
		},
		{
			name: "SQLite only supports B-tree",
			annotations: &ResourceAnnotations{
				IsResource: true,
				Fields: map[string]*FieldAnnotations{
					"Content": {
						FieldName: "Content",
						Index: &IndexConfig{
							Type: IndexTypeGIN,
						},
					},
				},
			},
			dbDriver:    "sqlite3",
			expectError: true,
			errorMsg:    "SQLite only supports B-tree indexes",
		},
		{
			name: "MySQL doesn't support GiST",
			annotations: &ResourceAnnotations{
				IsResource: true,
				Fields: map[string]*FieldAnnotations{
					"Location": {
						FieldName: "Location",
						Index: &IndexConfig{
							Type: IndexTypeGiST,
						},
					},
				},
			},
			dbDriver:    "mysql",
			expectError: true,
			errorMsg:    "does not support GiST indexes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForDatabase(tt.annotations, tt.dbDriver)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

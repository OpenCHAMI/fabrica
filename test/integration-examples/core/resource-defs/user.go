// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package resourcedefs

// UserSpec is a user account, modeled on UserSpec from
// examples/12-storage-annotations with the +fabrica:field annotations removed.
// Without them no field is immutable, unique, indexed, hashed, sensitive, or
// defaulted; only the validate tags apply.
type UserSpec struct {
	Username string `json:"username" validate:"required,min=3,max=32,alphanum"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	FullName string `json:"fullName" validate:"required,min=1,max=128"`
	Role     string `json:"role" validate:"required,oneof=admin user readonly"`
	APIKey   string `json:"apiKey,omitempty" validate:"omitempty,min=32,max=128"`
	Active   bool   `json:"active"`
}

// UserStatus defines the observed state of a user account.
type UserStatus struct {
	Phase     string `json:"phase,omitempty"`
	Message   string `json:"message,omitempty"`
	LastLogin string `json:"lastLogin,omitempty"`
}

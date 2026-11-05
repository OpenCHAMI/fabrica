// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package resourcedefs

// UserSpec is a system user with authentication credentials. It exercises the
// storage annotations: dedicated (hybrid) storage where each annotated
// scalar spec field is promoted to a real spec_* column, plus a composite
// index over username+email.
//
// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:index:fields=Username,Email:name=idx_user_login:unique:type=btree
type UserSpec struct {
	// Username is the unique identifier for this user; immutable after creation.
	// +fabrica:field:immutable
	// +fabrica:field:unique
	// +fabrica:field:index
	Username string `json:"username" validate:"required,min=3,max=32,alphanum"`

	// Email must be unique across all users.
	// +fabrica:field:unique
	// +fabrica:field:index
	Email string `json:"email" validate:"required,email"`

	// Password is plaintext in the request and bcrypt-hashed before storage.
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Password string `json:"password" validate:"required,min=8"`

	// FullName is the display name; not annotated, so it stays in spec_data JSON.
	FullName string `json:"fullName" validate:"required,min=1,max=128"`

	// Role defines the permission level.
	// +fabrica:field:default=user
	// +fabrica:field:index
	Role string `json:"role" validate:"required,oneof=admin user readonly"`

	// Active indicates whether the account is enabled.
	// +fabrica:field:default=true
	Active bool `json:"active"`
}

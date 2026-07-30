// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"encoding/json"
	"time"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type User struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   fabrica.Metadata `json:"metadata"`
	Spec       UserSpec         `json:"spec"`
	Status     UserStatus       `json:"status,omitempty"`
}

type UserSpec struct {
	// +fabrica:field:immutable
	Username string `json:"username" validate:"required"`
	// +fabrica:field:unique
	Email string `json:"email" validate:"required,email"`
	// +fabrica:field:storage=hashed:bcrypt:cost=4
	// +fabrica:field:sensitive
	Password string `json:"password" validate:"omitempty,min=8"`
	// +fabrica:field:sensitive
	RecoveryHint string `json:"recoveryHint"`
	// +fabrica:field:default=user
	// +fabrica:field:index=btree
	Role *string `json:"role,omitempty"`
	// +fabrica:field:default=true
	Active *bool `json:"active,omitempty"`
	// +fabrica:field:default=3
	Retries *int `json:"retries,omitempty"`
	// +fabrica:field:default=-1
	Quota *int64 `json:"quota,omitempty"`
	// +fabrica:field:default=1.5
	Score      *float64  `json:"score,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
	Aliases    []string  `json:"aliases"`
}

type UserStatus struct {
	State      string `json:"state"`
	LoginCount int    `json:"loginCount"`
}

func (u *User) Validate(context.Context) error { return nil }
func (u *User) GetKind() string                { return "User" }
func (u *User) GetName() string                { return u.Metadata.Name }
func (u *User) GetUID() string                 { return u.Metadata.UID }
func (u *User) IsHub()                         {}

func (u User) MarshalJSON() ([]byte, error) {
	type plain User
	redacted := plain(u)
	redacted.Spec.Password = ""
	redacted.Spec.RecoveryHint = ""
	return json.Marshal(redacted)
}

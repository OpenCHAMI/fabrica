// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const dedicatedSecurityTokenSource = `package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       TokenSpec        ` + "`json:\"spec\"`" + `
	Status     TokenStatus      ` + "`json:\"status,omitempty\"`" + `
}

type TokenSpec struct {
	// +fabrica:field:unique
	DisplayName string ` + "`json:\"displayName\" validate:\"required\"`" + `
	Inventory []string ` + "`json:\"inventory,omitempty\"`" + `

	// +fabrica:field:storage=hashed:bcrypt:cost=4
	Password string ` + "`json:\"password\" validate:\"required\"`" + `

	// +fabrica:field:storage=hashed:bcrypt:cost=4
	OptionalKey string ` + "`json:\"optionalKey,omitempty\" validate:\"omitempty\"`" + `

	// +fabrica:field:storage=hashed:bcrypt:cost=4
	// +fabrica:field:immutable
	ImmutableSecret string ` + "`json:\"immutableSecret\" validate:\"required\"`" + `

	// +fabrica:field:sensitive
	SensitiveNote string ` + "`json:\"sensitiveNote\" validate:\"required\"`" + `

	// +fabrica:field:sensitive
	SensitiveBool bool ` + "`json:\"sensitiveBool,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveInt int ` + "`json:\"sensitiveInt,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveInt64 int64 ` + "`json:\"sensitiveInt64,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveFloat64 float64 ` + "`json:\"sensitiveFloat64,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveTime time.Time ` + "`json:\"sensitiveTime,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveStrings []string ` + "`json:\"sensitiveStrings,omitempty\"`" + `

	// +fabrica:field:sensitive
	SensitiveStringPtr *string ` + "`json:\"sensitiveStringPtr,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveBoolPtr *bool ` + "`json:\"sensitiveBoolPtr,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveIntPtr *int ` + "`json:\"sensitiveIntPtr,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveInt64Ptr *int64 ` + "`json:\"sensitiveInt64Ptr,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveFloat64Ptr *float64 ` + "`json:\"sensitiveFloat64Ptr,omitempty\"`" + `
	// +fabrica:field:sensitive
	SensitiveTimePtr *time.Time ` + "`json:\"sensitiveTimePtr,omitempty\"`" + `
}

type TokenStatus struct {
	State       string ` + "`json:\"state,omitempty\"`" + `
	FailMarshal bool   ` + "`json:\"-\"`" + `
}

func (s TokenStatus) MarshalJSON() ([]byte, error) {
	if s.FailMarshal {
		return nil, fmt.Errorf("forced status marshal failure")
	}
	type tokenStatus TokenStatus
	return json.Marshal(tokenStatus(s))
}

func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string                { return "Token" }
func (r *Token) GetName() string                { return r.Metadata.Name }
func (r *Token) GetUID() string                 { return r.Metadata.UID }
func (r *Token) IsHub()                         {}
`

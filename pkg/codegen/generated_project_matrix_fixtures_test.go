// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

const generatedAllTypesNoHashSource = `package v1

import (
	"context"
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
	// +fabrica:field:default=ready
	Text string ` + "`json:\"text\"`" + `
	// +fabrica:field:default=true
	Enabled bool ` + "`json:\"enabled\"`" + `
	// +fabrica:field:default=7
	Count int ` + "`json:\"count\"`" + `
	// +fabrica:field:default=-9
	Sequence int64 ` + "`json:\"sequence\"`" + `
	// +fabrica:field:default=2.5
	Ratio float64 ` + "`json:\"ratio\"`" + `
	ObservedAt time.Time ` + "`json:\"observedAt\"`" + `
	Aliases []string ` + "`json:\"aliases\"`" + `
	Description *string ` + "`json:\"description\"`" + `
	Ready *bool ` + "`json:\"ready\"`" + `
	Limit *int ` + "`json:\"limit\"`" + `
	Offset *int64 ` + "`json:\"offset\"`" + `
	Threshold *float64 ` + "`json:\"threshold\"`" + `
	ExpiresAt *time.Time ` + "`json:\"expiresAt\"`" + `
	// +fabrica:field:index=btree
	Lookup string ` + "`json:\"lookup\" validate:\"required\"`" + `
	// +fabrica:field:unique
	Code string ` + "`json:\"code\" validate:\"required\"`" + `
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Secret string ` + "`json:\"secret\" validate:\"required\"`" + `
}

type TokenStatus struct { State string ` + "`json:\"state\"`" + ` }
func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string { return "Token" }
func (r *Token) GetName() string { return r.Metadata.Name }
func (r *Token) GetUID() string { return r.Metadata.UID }
func (r *Token) IsHub() {}
`

const generatedPostgreSQLBcryptSource = `package v1

import (
	"context"

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
	// +fabrica:field:index=gin
	Tags []string ` + "`json:\"tags\"`" + `
	// +fabrica:field:index=hash
	Fingerprint *string ` + "`json:\"fingerprint\"`" + `
	// +fabrica:field:index=btree
	// +fabrica:field:unique
	Slug string ` + "`json:\"slug\" validate:\"required\"`" + `
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	Password string ` + "`json:\"password\" validate:\"required\"`" + `
}

type TokenStatus struct{}
func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string { return "Token" }
func (r *Token) GetName() string { return r.Metadata.Name }
func (r *Token) GetUID() string { return r.Metadata.UID }
func (r *Token) IsHub() {}
`

const generatedUnsupportedFieldSource = `package v1

import (
	"context"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	APIVersion string
	Kind string
	Metadata fabrica.Metadata
	Spec TokenSpec
	Status TokenStatus
}
type TokenSpec struct {
	// +fabrica:field:sensitive
	Payload map[string]string ` + "`json:\"payload\"`" + `
}
type TokenStatus struct{}
func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string { return "Token" }
func (r *Token) GetName() string { return r.Metadata.Name }
func (r *Token) GetUID() string { return r.Metadata.UID }
func (r *Token) IsHub() {}
`

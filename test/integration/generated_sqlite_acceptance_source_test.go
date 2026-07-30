// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const generatedSQLiteTokenSource = `package v1

import (
	"context"
	"encoding/json"
	"errors"

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
	// +fabrica:field:index=btree
	Lookup string ` + "`json:\"lookup\" validate:\"required\"`" + `
	// +fabrica:field:unique
	Slug string ` + "`json:\"slug\" validate:\"required\"`" + `
	// +fabrica:field:default=false
	Enabled bool ` + "`json:\"enabled\"`" + `
	// +fabrica:field:default=0
	Retries int ` + "`json:\"retries\"`" + `
	// +fabrica:field:immutable
	ImmutableCode string ` + "`json:\"immutableCode\" validate:\"required\"`" + `
	// +fabrica:field:default=fallback
	OptionalNote *string ` + "`json:\"optionalNote,omitempty\"`" + `
}

type TokenStatus struct { State string ` + "`json:\"state\"`" + ` }

func (s *TokenStatus) UnmarshalJSON(data []byte) error {
	type plain TokenStatus
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil { return err }
	if decoded.State == "corrupt" { return errors.New("corrupt token status") }
	*s = TokenStatus(decoded)
	return nil
}

func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string                { return "Token" }
func (r *Token) GetName() string                { return r.Metadata.Name }
func (r *Token) GetUID() string                 { return r.Metadata.UID }
func (r *Token) IsHub()                         {}
`

const generatedSQLiteGINTokenSource = `package v1

import (
	"context"
	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	APIVersion string ` + "`json:\"apiVersion\"`" + `
	Kind string ` + "`json:\"kind\"`" + `
	Metadata fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec TokenSpec ` + "`json:\"spec\"`" + `
	Status TokenStatus ` + "`json:\"status,omitempty\"`" + `
}
type TokenSpec struct {
	// +fabrica:field:index=gin
	Tags []string ` + "`json:\"tags\"`" + `
}
type TokenStatus struct{}
func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string { return "Token" }
func (r *Token) GetName() string { return r.Metadata.Name }
func (r *Token) GetUID() string { return r.Metadata.UID }
func (r *Token) IsHub() {}
`

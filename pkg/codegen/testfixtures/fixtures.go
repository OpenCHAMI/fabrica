// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package testfixtures contains test fixtures for the codegen package.
package testfixtures

import (
	"time"

	"github.com/openchami/fabrica/pkg/resource"
)

// MappedToken exercises dedicated Ent schema and adapter type mappings.
type MappedToken struct {
	resource.Resource
	Spec MappedTokenSpec `json:"spec"`
}

// MappedTokenSpec contains the supported mapped field type matrix.
type MappedTokenSpec struct {
	Subject        string            `json:"subject" validate:"required"`
	UsageCount     int               `json:"usage_count"`
	Revoked        bool              `json:"revoked"`
	SequenceNumber int64             `json:"sequence_number"`
	Weight         float64           `json:"weight"`
	TTL            time.Duration     `json:"ttl"`
	IssuedAt       time.Time         `json:"issued_at" validate:"required"`
	ConsumedAt     *time.Time        `json:"consumed_at"`
	Scopes         []string          `json:"scopes"`
	Fingerprint    []byte            `json:"fingerprint"`
	ReplayAttempts []time.Time       `json:"replay_attempts"`
	Labels         map[string]string `json:"labels"`
}

// AliasSubject is a named string fixture for adapter conversion tests.
type AliasSubject string

// AliasUsageCount is a named int fixture for adapter conversion tests.
type AliasUsageCount int

// AliasRevoked is a named bool fixture for adapter conversion tests.
type AliasRevoked bool

// AliasSequenceNumber is a named int64 fixture for adapter conversion tests.
type AliasSequenceNumber int64

// AliasWeight is a named float64 fixture for adapter conversion tests.
type AliasWeight float64

// AliasToken exercises generated adapters for named scalar field aliases.
type AliasToken struct {
	resource.Resource
	Spec AliasTokenSpec `json:"spec"`
}

// AliasTokenSpec contains named scalar field aliases used by codegen tests.
type AliasTokenSpec struct {
	Subject        AliasSubject        `json:"subject" validate:"required"`
	UsageCount     AliasUsageCount     `json:"usage_count"`
	Revoked        AliasRevoked        `json:"revoked"`
	SequenceNumber AliasSequenceNumber `json:"sequence_number"`
	Weight         AliasWeight         `json:"weight"`
}

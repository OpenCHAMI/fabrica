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

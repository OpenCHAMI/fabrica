// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

// Package revision defines shared immutable-resource revision contracts.
package revision

import (
	"errors"
	"time"
)

// UIDExample is a TypeID-style revision UID with the revision-scoped prefix.
const UIDExample = "rev_01h2xcejqtf2nbrexx3vqjhp41"

// Selector identifies a movable revision alias.
type Selector string

const (
	// SelectorDefault resolves the explicitly promoted revision.
	SelectorDefault Selector = "default"
	// SelectorLatest resolves the most recently created revision.
	SelectorLatest Selector = "latest"
)

var (
	// ErrInvalidReference indicates that a reference does not identify one series revision.
	ErrInvalidReference = errors.New("revision: invalid resource reference")
	// ErrUnsupportedSelector indicates that a reference uses an unknown selector.
	ErrUnsupportedSelector = errors.New("revision: unsupported selector")
)

// Reference identifies one revision in a named resource series.
type Reference struct {
	Name         string   `json:"name" yaml:"name"`
	UID          string   `json:"uid,omitempty" yaml:"uid,omitempty"`
	RevisionName string   `json:"revisionName,omitempty" yaml:"revisionName,omitempty"`
	Selector     Selector `json:"selector,omitempty" yaml:"selector,omitempty"`
}

// Validate verifies that the series name is present and at most one revision selector is set.
// An omitted selector is valid because a bare series name resolves to default.
func (r Reference) Validate() error {
	if r.Name == "" {
		return ErrInvalidReference
	}

	selected := 0
	if r.UID != "" {
		selected++
	}
	if r.RevisionName != "" {
		selected++
	}
	if r.Selector != "" {
		selected++
	}
	if selected > 1 {
		return ErrInvalidReference
	}

	if r.Selector != "" && r.Selector != SelectorDefault && r.Selector != SelectorLatest {
		return ErrUnsupportedSelector
	}
	return nil
}

// SelectorOrDefault returns default for a bare series-name reference.
func (r Reference) SelectorOrDefault() Selector {
	if r.UID == "" && r.RevisionName == "" && r.Selector == "" {
		return SelectorDefault
	}
	return r.Selector
}

// Metadata describes one immutable revision record.
type Metadata struct {
	UID          string     `json:"uid" yaml:"uid"`
	SeriesName   string     `json:"seriesName" yaml:"seriesName"`
	RevisionName string     `json:"revisionName,omitempty" yaml:"revisionName,omitempty"`
	Number       uint64     `json:"number" yaml:"number"`
	Digest       string     `json:"digest" yaml:"digest"`
	CreatedAt    time.Time  `json:"createdAt" yaml:"createdAt"`
	RetiredAt    *time.Time `json:"retiredAt,omitempty" yaml:"retiredAt,omitempty"`
}

// SeriesStatus records the movable aliases for a named resource series.
type SeriesStatus struct {
	Name               string    `json:"name" yaml:"name"`
	DefaultRevisionUID string    `json:"defaultRevisionUid" yaml:"defaultRevisionUid"`
	LatestRevisionUID  string    `json:"latestRevisionUid" yaml:"latestRevisionUid"`
	RevisionCount      uint64    `json:"revisionCount" yaml:"revisionCount"`
	UpdatedAt          time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// ResolveResponse reports the immutable revision selected from a series.
type ResolveResponse struct {
	SeriesName string   `json:"seriesName" yaml:"seriesName"`
	Selector   Selector `json:"selector,omitempty" yaml:"selector,omitempty"`
	Revision   Metadata `json:"revision" yaml:"revision"`
}

// ConflictResponse describes a deterministic revision-name content conflict.
type ConflictResponse struct {
	Error               string `json:"error" yaml:"error"`
	ExistingRevisionUID string `json:"existingRevisionUid,omitempty" yaml:"existingRevisionUid,omitempty"`
	ExistingDigest      string `json:"existingDigest,omitempty" yaml:"existingDigest,omitempty"`
	RequestedDigest     string `json:"requestedDigest,omitempty" yaml:"requestedDigest,omitempty"`
}

// PreconditionResponse describes a failed series compare-and-swap operation.
type PreconditionResponse struct {
	Error       string `json:"error" yaml:"error"`
	CurrentETag string `json:"currentEtag,omitempty" yaml:"currentEtag,omitempty"`
}

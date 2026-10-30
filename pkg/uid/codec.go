// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

// Package uid provides Fabrica's UID format boundary. It does not select the
// default UID format used by resources or change existing stored identifiers.
package uid

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.jetify.com/typeid/v2"
)

// Format identifies the external representation of a UID.
type Format string

const (
	// FormatLegacy identifies the historical prefix-hex representation.
	FormatLegacy Format = "legacy"
	// FormatTypeID identifies the TypeID specification 0.3.0 representation.
	FormatTypeID Format = "typeid"
)

// ParsedUID describes a UID without changing its original representation.
type ParsedUID struct {
	Format Format
	Prefix string
	Suffix string
	// UUID is the canonical UUID string for TypeIDs, or empty for legacy IDs.
	UUID string
}

// UUIDGenerator supplies UUID bytes for generation. Implementations must return
// RFC 9562 UUIDv7 values and be safe for any concurrent use of their codec.
type UUIDGenerator func() ([16]byte, error)

// Codec generates TypeID/UUIDv7 UIDs and parses TypeID and legacy UIDs.
// Its zero value is ready to use and is safe for concurrent use. A codec with a
// custom generator is safe for concurrent use if that generator is safe.
type Codec struct {
	generate UUIDGenerator
}

// NewCodec returns a codec with an instance-local UUID generator. A nil generator
// selects the default UUIDv7 generator. This hook supports deterministic tests
// without replacing process-wide randomness or clocks.
func NewCodec(generate UUIDGenerator) *Codec {
	return &Codec{generate: generate}
}

// ValidatePrefix checks TypeID 0.3.0 prefix rules. The empty prefix is valid in
// the specification; resource registration may impose a nonempty requirement.
func ValidatePrefix(prefix string) error {
	_, err := typeid.FromBytes(prefix, make([]byte, 16))
	if err != nil {
		return fmt.Errorf("uid: invalid TypeID prefix: %w", err)
	}
	return nil
}

// ResourcePrefix derives the UID prefix for a resource name and checks it
// against TypeID 0.3.0 rules. The derived prefix is the lowercase name.
//
// Unlike ValidatePrefix, the empty prefix is rejected: a resource needs a
// nonempty prefix to generate UIDs. On failure the derived prefix is returned
// alongside the error so callers can name it in their own diagnostics; that
// value is empty only when the name itself is empty.
func ResourcePrefix(name string) (string, error) {
	prefix := strings.ToLower(name)
	if prefix == "" {
		return "", errors.New("uid: resource UID prefix must not be empty")
	}
	return prefix, ValidatePrefix(prefix)
}

// Generate returns a TypeID containing a UUIDv7 with the RFC 9562 variant.
// Prefix validation runs before the generator is invoked.
func (c Codec) Generate(prefix string) (string, error) {
	if err := ValidatePrefix(prefix); err != nil {
		return "", err
	}
	generate := c.generate
	if generate == nil {
		generate = func() ([16]byte, error) {
			id, err := uuid.NewV7()
			return [16]byte(id), err
		}
	}
	bytes, err := generate()
	if err != nil {
		return "", fmt.Errorf("uid: generate UUIDv7: %w", err)
	}
	id := uuid.UUID(bytes)
	if id.Version() != 7 || id.Variant() != uuid.RFC4122 {
		return "", errors.New("uid: generator must return an RFC 9562 UUIDv7")
	}
	tid, err := typeid.FromBytes(prefix, bytes[:])
	if err != nil {
		return "", fmt.Errorf("uid: encode TypeID: %w", err)
	}
	return tid.String(), nil
}

// Parse accepts TypeID 0.3.0 strings and legacy prefix-hex strings. TypeID
// parsing accepts all UUID payloads allowed by the specification, not only v7.
// Legacy parsing preserves resource.ParseUID's historical behavior: exactly
// one hyphen and a hex-decodable suffix, without prefix validation or nonempty
// checks. Parsing does not imply registration and never logs or rewrites IDs.
func (c Codec) Parse(value string) (ParsedUID, error) {
	if strings.Contains(value, "-") {
		parts := strings.Split(value, "-")
		if len(parts) != 2 {
			return ParsedUID{}, errors.New("uid: invalid legacy UID: expected prefix-hex")
		}
		if _, err := hex.DecodeString(parts[1]); err != nil {
			return ParsedUID{}, fmt.Errorf("uid: invalid legacy hex suffix: %w", err)
		}
		return ParsedUID{Format: FormatLegacy, Prefix: parts[0], Suffix: parts[1]}, nil
	}
	tid, err := typeid.Parse(value)
	if err != nil {
		return ParsedUID{}, fmt.Errorf("uid: invalid TypeID: %w", err)
	}
	return ParsedUID{
		Format: FormatTypeID,
		Prefix: tid.Prefix(),
		Suffix: tid.Suffix(),
		UUID:   tid.UUID(),
	}, nil
}

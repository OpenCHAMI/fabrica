// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"testing"

	"github.com/google/uuid"
	"github.com/openchami/fabrica/pkg/uid"
)

func TestNewUIDUsesSharedCodec(t *testing.T) {
	value, err := NewUID()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := (uid.Codec{}).Parse(value)
	if err != nil || parsed.Format != uid.FormatTypeID || parsed.Prefix != "rev" {
		t.Fatalf("revision UID = %q, %+v, %v", value, parsed, err)
	}
	id, err := uuid.Parse(parsed.UUID)
	if err != nil || id.Version() != 7 || id.Variant() != uuid.RFC4122 {
		t.Fatalf("revision payload = %q, %v", parsed.UUID, err)
	}
}

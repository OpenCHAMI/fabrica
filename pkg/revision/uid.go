// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"fmt"

	"github.com/google/uuid"
)

const typeIDAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// NewUID returns a revision-scoped TypeID whose payload is a UUIDv7.
func NewUID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("revision: generate UUIDv7: %w", err)
	}

	encoded := make([]byte, 26)
	for outputBit := 0; outputBit < 130; outputBit += 5 {
		value := byte(0)
		for bit := 0; bit < 5; bit++ {
			value <<= 1
			inputBit := outputBit + bit - 2
			if inputBit >= 0 && id[inputBit/8]&(1<<uint(7-inputBit%8)) != 0 {
				value |= 1
			}
		}
		encoded[outputBit/5] = typeIDAlphabet[value]
	}

	return "rev_" + string(encoded), nil
}

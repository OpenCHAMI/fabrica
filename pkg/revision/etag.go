// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ValueETag returns a strong ETag for a JSON-serializable revision response.
func ValueETag(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("revision: encode ETag value: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return `"sha256:` + hex.EncodeToString(sum[:]) + `"`, nil
}

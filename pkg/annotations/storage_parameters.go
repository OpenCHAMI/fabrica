// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import "fmt"

func parseSingleStorageParameter(parts []string, allowedKey string) (string, bool, error) {
	value := ""
	present := false
	for _, part := range parts {
		key, candidate, hasValue := ParseKeyValue(part)
		if !hasValue || key == "" || candidate == "" {
			return "", false, fmt.Errorf("storage parameter %q must use key=value syntax", part)
		}
		if key != allowedKey {
			return "", false, fmt.Errorf("unknown storage parameter %q", key)
		}
		if present {
			return "", false, fmt.Errorf("duplicate storage parameter %q", key)
		}
		value = candidate
		present = true
	}
	return value, present, nil
}

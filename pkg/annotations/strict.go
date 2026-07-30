// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"strings"
)

var resourceDirectiveKeys = []string{"resource", "storage", "verbs", "exposure"}
var fieldDirectiveKeys = []string{"sensitive", "immutable", "unique", "storage", "index", "default"}

func strictAnnotationParts(annotation string) ([]string, error) {
	raw := strings.TrimSpace(annotation)
	raw = strings.TrimPrefix(raw, "+fabrica:")
	if raw == "" || strings.HasSuffix(raw, ":") || strings.Contains(raw, "::") {
		return nil, fmt.Errorf("malformed directive")
	}
	parts := strings.Split(raw, ":")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "" {
			return nil, fmt.Errorf("malformed directive")
		}
	}
	return parts, nil
}

func directiveKey(part string) string {
	key, _, _ := ParseKeyValue(part)
	return key
}

func recordDirective(seen map[string]string, key, directive string) error {
	previous, exists := seen[key]
	if !exists {
		seen[key] = directive
		return nil
	}
	return fmt.Errorf("duplicate or conflicting %q directive; previous directive was %q", key, previous)
}

func unknownDirectiveError(scope, key string, known []string, directive string) *ParseError {
	message := fmt.Sprintf("unknown %s directive %q", scope, key)
	suggestion := nearestDirective(key, known)
	if suggestion != "" {
		message += fmt.Sprintf("; did you mean %q?", suggestion)
	}
	result := parseError(parseSource{directive: directive}, message, nil)
	result.Suggestion = suggestion
	return result
}

func nearestDirective(value string, known []string) string {
	best := ""
	bestDistance := len(value) + 1
	for _, candidate := range known {
		distance := editDistance(value, candidate)
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	if bestDistance > 2 {
		return ""
	}
	return best
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for leftIndex, leftRune := range left {
		current := make([]int, len(right)+1)
		current[0] = leftIndex + 1
		for rightIndex, rightRune := range right {
			cost := 1
			if leftRune == rightRune {
				cost = 0
			}
			current[rightIndex+1] = min(
				current[rightIndex]+1,
				previous[rightIndex+1]+1,
				previous[rightIndex]+cost,
			)
		}
		previous = current
	}
	return previous[len(right)]
}

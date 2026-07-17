// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"strconv"
	"strings"
)

// Directive represents a parsed annotation directive
type Directive struct {
	raw   string
	parts []string
	opts  map[string]string
}

// ParseDirective parses an annotation string into structured parts
// Example: "+fabrica:field:storage=hashed:bcrypt:cost=12"
//   - parts: ["field", "storage=hashed", "bcrypt", "cost=12"]
//   - opts: {"storage": "hashed", "cost": "12"}
func ParseDirective(raw string) (*Directive, error) {
	d := &Directive{
		raw:  raw,
		opts: make(map[string]string),
	}

	// Remove prefix
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "//")
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "+fabrica:")

	if raw == "" {
		return nil, fmt.Errorf("empty directive")
	}

	// Split by :
	d.parts = strings.Split(raw, ":")

	// Extract key=value pairs into opts map
	for _, part := range d.parts {
		if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
			d.opts[kv[0]] = kv[1]
		}
	}

	return d, nil
}

// GetPart returns the part at the given index, or empty string if out of bounds
func (d *Directive) GetPart(idx int) string {
	if idx < 0 || idx >= len(d.parts) {
		return ""
	}
	return d.parts[idx]
}

// HasPart returns true if the part at index exists and is non-empty
func (d *Directive) HasPart(idx int) bool {
	return d.GetPart(idx) != ""
}

// GetOpt returns the value for a given key, and whether it exists
func (d *Directive) GetOpt(key string) (string, bool) {
	v, ok := d.opts[key]
	return v, ok
}

// GetOptString returns the string value for a key, or defaultVal if not found
func (d *Directive) GetOptString(key string, defaultVal string) string {
	if v, ok := d.opts[key]; ok {
		return v
	}
	return defaultVal
}

// GetOptInt returns the int value for a key, or defaultVal if not found or invalid
func (d *Directive) GetOptInt(key string, defaultVal int) (int, error) {
	v, ok := d.opts[key]
	if !ok {
		return defaultVal, nil
	}

	intVal, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal, fmt.Errorf("invalid integer value %q for key %q: %w", v, key, err)
	}

	return intVal, nil
}

// GetOptBool returns the bool value for a key, or defaultVal if not found
// Recognizes: true/false, yes/no, 1/0
func (d *Directive) GetOptBool(key string, defaultVal bool) (bool, error) {
	v, ok := d.opts[key]
	if !ok {
		return defaultVal, nil
	}

	switch strings.ToLower(v) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return defaultVal, fmt.Errorf("invalid boolean value %q for key %q", v, key)
	}
}

// Parts returns all parts
func (d *Directive) Parts() []string {
	return d.parts
}

// Opts returns all options
func (d *Directive) Opts() map[string]string {
	return d.opts
}

// Raw returns the original raw string
func (d *Directive) Raw() string {
	return d.raw
}

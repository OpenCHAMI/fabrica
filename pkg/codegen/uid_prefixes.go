// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"
	"strings"

	"github.com/openchami/fabrica/pkg/uid"
)

// ValidateTypeIDPrefixes audits the lowercase resource names emitted as UID
// prefixes by the routes template. It reports all incompatible names without
// renaming prefixes or changing registration. Generation does not call this
// opt-in preflight until TypeID becomes the default resource UID format.
// This does not audit custom registrations in user-authored resource files.
func (g *Generator) ValidateTypeIDPrefixes() error {
	var problems []error
	for _, r := range g.Resources {
		prefix := strings.ToLower(r.Name)
		if prefix == "" {
			problems = append(problems, errors.New("resource UID prefix must not be empty"))
			continue
		}
		if err := uid.ValidatePrefix(prefix); err != nil {
			problems = append(problems, fmt.Errorf("resource %q generates UID prefix %q: %w", r.Name, prefix, err))
		}
	}
	return errors.Join(problems...)
}

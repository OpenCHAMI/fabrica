// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"

	"github.com/openchami/fabrica/pkg/uid"
)

// ValidateTypeIDPrefixes audits the lowercase resource names emitted as UID
// prefixes by the routes template. It reports all incompatible names without
// renaming prefixes or changing registration. Server generation calls this preflight
// before emitting files.
// This does not audit custom registrations in user-authored resource files;
// generated services warn about those at startup instead, from
// warnIncompatibleUIDPrefixes in routes_generated.go.
func (g *Generator) ValidateTypeIDPrefixes() error {
	var problems []error
	for _, r := range g.Resources {
		prefix, err := uid.ResourcePrefix(r.Name)
		switch {
		case err == nil:
		case prefix == "":
			problems = append(problems, errors.New("resource UID prefix must not be empty"))
		default:
			problems = append(problems, fmt.Errorf("resource %q generates UID prefix %q: %w", r.Name, prefix, err))
		}
	}
	return errors.Join(problems...)
}

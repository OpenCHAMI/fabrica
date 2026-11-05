// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"github.com/openchami/fabrica/pkg/annotations"
)

// LoadResourceAnnotations attaches source comments to registered resources.
// Call after registration, from the project root, before running emitters.
// Unlike reflection-only registration this requires available project sources.
func (g *Generator) LoadResourceAnnotations() error {
	for i := range g.Resources {
		resource := &g.Resources[i]
		relative, ok := strings.CutPrefix(resource.Package, g.ModulePath+"/")
		if !ok {
			return fmt.Errorf("load annotations for %s: package %q must be inside project module %q", resource.Name, resource.Package, g.ModulePath)
		}
		dir := filepath.FromSlash(relative)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("load annotations for %s in %s: %w", resource.Name, dir, err)
		}
		source := ""
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			match, err := build.Default.MatchFile(dir, entry.Name())
			if err != nil {
				return fmt.Errorf("load annotations for %s: %w", resource.Name, err)
			}
			if match {
				source = filepath.Join(dir, entry.Name())
				break
			}
		}
		if source == "" {
			return fmt.Errorf("load annotations for %s: no buildable Go source in %s", resource.Name, dir)
		}
		annots, err := g.ParseResourceAnnotations(source, resource.Name)
		if err != nil {
			return fmt.Errorf("load annotations for %s: %w", resource.Name, err)
		}
		if len(annots.RawAnnotations) == 0 && len(annots.Fields) == 0 {
			continue
		}
		// Registration already establishes that this is a resource. A missing marker
		// must not bypass semantic validation of storage or field annotations.
		annots.IsResource = true
		if err = annotations.Validate(annots); err != nil {
			return fmt.Errorf("load annotations for %s: %w", resource.Name, err)
		}
		if annots.StorageMode == annotations.StorageModeDedicated {
			if g.StorageType != "ent" {
				return fmt.Errorf("resource %s: dedicated storage requires the ent backend", resource.Name)
			}
			candidate := *resource
			candidate.Annotations = annots
			if err = g.validateDedicatedEntResource(candidate); err != nil {
				return err
			}
		}
		resource.Annotations = annots
	}
	return nil
}

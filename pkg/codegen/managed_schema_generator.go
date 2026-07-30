// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// GenerateEntSchemas atomically replaces the managed generic and dedicated Ent schema tree.
func (g *Generator) GenerateEntSchemas() error {
	if g.StorageType != "ent" {
		return nil
	}

	fmt.Printf("🗄️  Generating Ent schemas...\n")
	schemaDir := filepath.Join("internal", "storage", "ent", "schema")
	output := newManagedSchemaOutput(schemaDir, newManagedSchemaOperations(g.renderManagedSchemaTemplate))
	return output.withTransactionLock(func() error {
		if err := output.recoverLocked(); err != nil {
			return fmt.Errorf("recover managed Ent schema tree: %w", err)
		}
		return g.generateEntSchemasLocked(output, schemaDir)
	})
}

func (g *Generator) generateEntSchemasLocked(output *managedSchemaOutput, schemaDir string) error {
	dedicatedSchemas, err := g.prepareDedicatedSchemas(schemaDir)
	if err != nil {
		var preparationErr *dedicatedSchemaPreparationError
		if errors.As(err, &preparationErr) {
			name := strings.ToLower(preparationErr.resourceName) + ".go"
			if quarantineErr := output.quarantineLocked(name); quarantineErr != nil {
				return errors.Join(err, quarantineErr)
			}
		}
		return err
	}

	files := []managedSchemaFile{
		{name: "resource.go", templateName: "entSchemaResource"},
		{name: "label.go", templateName: "entSchemaLabel"},
		{name: "annotation.go", templateName: "entSchemaAnnotation"},
	}
	for _, schema := range dedicatedSchemas {
		files = append(files, managedSchemaFile{
			name: filepath.Base(schema.outputPath), templateName: "entSchemaResourceDedicated", data: schema.data,
		})
	}
	if err := output.commitLocked(files); err != nil {
		return fmt.Errorf("commit managed Ent schema tree: %w", err)
	}
	for _, schema := range dedicatedSchemas {
		fmt.Printf("  ✓ Generated dedicated schema for %s\n", schema.resourceName)
	}
	return nil
}

func (g *Generator) renderManagedSchemaTemplate(templateName string, data interface{}) ([]byte, error) {
	tmpl, exists := g.Templates[templateName]
	if !exists {
		return nil, fmt.Errorf("template %s not found", templateName)
	}
	if data == nil {
		data = g.commonTemplateData(templateName)
	} else if dataMap, ok := data.(map[string]interface{}); ok {
		data = g.mergeCommonTemplateData(templateName, dataMap)
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", templateName, err)
	}
	return buffer.Bytes(), nil
}

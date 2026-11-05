// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
// SPDX-License-Identifier: MIT

package codegen

import (
	"github.com/openchami/fabrica/pkg/annotations"
	"testing"
)

func TestDedicatedEntIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		names   []string
		wantErr bool
	}{
		{"aliases", []string{"display_text", "apiKey", "request_id"}, false},
		{"metadata", []string{"name"}, true},
		{"implicit ID", []string{"id"}, true},
		{"equivalent names", []string{"full_name", "Full_name"}, true},
		{"invalid identifier", []string{"display.text"}, true},
		{"unicode", []string{"élan"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := annotations.NewResourceAnnotations()
			a.IsResource = true
			a.StorageMode = annotations.StorageModeDedicated
			a.Fields["Value"] = annotations.NewFieldAnnotations("Value")
			r := ResourceMetadata{Name: "Token", Annotations: a}
			for _, name := range tc.names {
				r.SpecFields = append(r.SpecFields, SpecField{Name: "Value", JSONName: name, Type: "string"})
			}
			g := NewGenerator(".", "test", "test")
			g.DBDriver = "sqlite"
			if err := g.validateDedicatedEntResource(r); (err != nil) != tc.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

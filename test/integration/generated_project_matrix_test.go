// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import "testing"

func TestGeneratedProjectMatrix_passes_generation_vet_and_build(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		source string
		mixed  bool
	}{
		{name: "SQLite all types no hash portable index and mixed storage", driver: "sqlite", source: generatedAllTypesNoHashSource, mixed: true},
		{name: "PostgreSQL bcrypt GIN hash and B-tree indexes", driver: "postgres", source: generatedPostgreSQLBcryptSource},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			project := newGeneratedProject(t, "ent")
			project.setDBDriver(t, test.driver)
			project.writeResourceSource(t, test.source)
			if test.mixed {
				project.writeAPIsResources(t, "Token", "Device")
				project.writeNamedResourceSource(t, "device_types.go", generatedDeviceSource)
			}

			// When / Then
			project.requireGeneratedProjectGate(t, generatedProjectExpectations{
				dedicatedResources: []string{"Token"},
				genericResources: func() []string {
					if test.mixed {
						return []string{"Device"}
					}
					return nil
				}(),
			})
		})
	}
}

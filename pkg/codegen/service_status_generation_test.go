// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateServiceStatusSupport(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	gen := NewGenerator(outDir, "main", "example.com/status-service")
	gen.Resources = []ResourceMetadata{
		{
			Name:         "Node",
			PluralName:   "nodes",
			Package:      "example.com/status-service/apis/v1",
			PackageAlias: "v1",
			TypeName:     "*v1.Node",
			SpecType:     "v1.NodeSpec",
			StatusType:   "v1.NodeStatus",
			URLPath:      "/nodes",
			StorageName:  "Node",
		},
	}

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	for name, generate := range map[string]func() error{
		"models":  gen.GenerateModels,
		"routes":  gen.GenerateRoutes,
		"openapi": gen.GenerateOpenAPI,
	} {
		t.Run(name, func(t *testing.T) {
			if err := generate(); err != nil {
				t.Fatalf("generate %s: %v", name, err)
			}
		})
	}

	assertGeneratedFileContains(t, filepath.Join(outDir, "models_generated.go"), []string{
		"type ServiceStatus string",
		"ServiceStatusOK",
		"ServiceStatusDegraded",
		"ServiceStatusDown",
		"ServiceStatusInitializing",
		`ServiceStatus = "ok"`,
		`ServiceStatus = "degraded"`,
		`ServiceStatus = "down"`,
		`ServiceStatus = "initializing"`,
		"type ServiceStatusResponse struct",
		"`json:\"status\"`",
		"`json:\"service\"`",
	})

	assertGeneratedFileContains(t, filepath.Join(outDir, "routes_generated.go"), []string{
		"func RegisterGeneratedPublicRoutes(r chi.Router)",
		`r.Get("/service/status", serviceStatusHandler)`,
		"type serviceStatusProviderFunc func() ServiceStatus",
		"var serviceStatusProvider serviceStatusProviderFunc = func() ServiceStatus",
		"func serviceStatusHandler(w http.ResponseWriter, _ *http.Request)",
		"status := serviceStatusProvider()",
		"respondJSON(w, serviceStatusHTTPCode(status), ServiceStatusResponse{",
		"Status:  status",
		`"status-service"`,
		"func serviceStatusHTTPCode(status ServiceStatus) int",
		"return http.StatusServiceUnavailable",
		"Build metadata is intentionally excluded",
	})

	assertGeneratedFileContains(t, filepath.Join(outDir, "openapi_generated.go"), []string{
		`spec.Components.Schemas["ServiceStatusResponse"]`,
		`serviceStatusValueSchema.Enum = []interface{}{`,
		`"ok"`,
		`"degraded"`,
		`"down"`,
		`"initializing"`,
		`serviceStatusOp.Responses.Set("200"`,
		`serviceStatusOp.Responses.Set("503"`,
		`spec.Paths.Set("/service/status"`,
		`"service": "status-service"`,
		"excludes build metadata until an authenticated extension",
	})
}

func assertGeneratedFileContains(t *testing.T, path string, markers []string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file %s: %v", path, err)
	}

	for _, marker := range markers {
		if !strings.Contains(string(content), marker) {
			t.Errorf("generated file %s missing %q", path, marker)
		}
	}
}

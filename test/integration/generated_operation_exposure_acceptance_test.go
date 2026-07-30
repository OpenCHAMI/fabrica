// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedOperationExposureSurfaces_compile_and_filter(t *testing.T) {
	tests := []struct {
		name               string
		sourcePrefix       string
		directives         string
		wantHandlers       []string
		forbidHandlers     []string
		wantRoute          string
		wantClient         []string
		forbidPublicClient string
		wantCLI            []string
		forbidCLI          []string
		wantHookFile       bool
	}{
		{
			name: "unannotated complete surface",
			wantHandlers: []string{
				"func GetTokens(", "func GetToken(", "func CreateToken(", "func UpdateToken(",
				"func PatchToken(", "func DeleteToken(", "func UpdateTokenStatus(", "func PatchTokenStatus(",
			},
			wantRoute: "registerTokenRoutes(r)",
			wantClient: []string{
				"func (c *Client) GetTokens(", "func (c *Client) GetToken(", "func (c *Client) CreateToken(",
				"func (c *Client) UpdateToken(", "func (c *Client) PatchToken(", "func (c *Client) DeleteToken(",
			},
			wantCLI:      []string{"tokenListCmd", "tokenGetCmd", "tokenCreateCmd", "tokenUpdateCmd", "tokenPatchCmd", "tokenDeleteCmd"},
			wantHookFile: true,
		},
		{
			name:           "read only",
			directives:     "// +fabrica:verbs=list,get\n",
			wantHandlers:   []string{"func GetTokens(", "func GetToken("},
			forbidHandlers: []string{"func CreateToken(", "func UpdateToken(", "func PatchToken(", "func DeleteToken("},
			wantRoute:      "registerTokenRoutes(r)",
			wantCLI:        []string{"tokenListCmd", "tokenGetCmd"},
			forbidCLI:      []string{"tokenCreateCmd", "tokenUpdateCmd", "tokenPatchCmd", "tokenDeleteCmd"},
			wantHookFile:   true,
		},
		{
			name:           "status only",
			directives:     "// +fabrica:verbs=statusUpdate,statusPatch\n",
			wantHandlers:   []string{"func UpdateTokenStatus(", "func PatchTokenStatus("},
			forbidHandlers: []string{"func GetToken(", "func CreateToken(", "func UpdateToken(", "func PatchToken("},
			wantClient:     []string{"func (c *Client) UpdateTokenStatus(", "func (c *Client) PatchTokenStatus("},
			forbidCLI:      []string{"tokenCmd", "tokenListCmd", "tokenGetCmd"},
			wantHookFile:   true,
		},
		{
			name:           "version only",
			sourcePrefix:   "// +fabrica:resource-versioning=enabled\n",
			directives:     "// +fabrica:verbs=versionList,versionGet,versionDelete\n",
			wantHandlers:   []string{"func ListTokenVersions(", "func GetTokenVersion(", "func DeleteTokenVersion("},
			forbidHandlers: []string{"func GetToken(", "func CreateToken(", "func UpdateToken(", "func PatchToken("},
			wantClient:     []string{"func (c *Client) ListTokenVersions(", "func (c *Client) GetTokenVersion(", "func (c *Client) DeleteTokenVersion("},
			wantCLI:        []string{"tokenVersionsCmd", "tokenVersionsListCmd", "tokenVersionsGetCmd", "tokenVersionsDeleteCmd"},
			wantHookFile:   true,
		},
		{
			name:               "none",
			directives:         "// +fabrica:verbs=none\n",
			forbidHandlers:     []string{"func GetToken", "func CreateToken", "func UpdateToken", "func DeleteToken"},
			forbidPublicClient: "func (c *Client) GetToken",
			forbidCLI:          []string{"tokenCmd", "tokenListCmd", "tokenGetCmd"},
		},
		{
			name:               "private default none",
			directives:         "// +fabrica:exposure=private\n",
			forbidHandlers:     []string{"func GetToken", "func CreateToken", "func UpdateToken", "func DeleteToken"},
			forbidPublicClient: "func (c *Client) GetToken",
			forbidCLI:          []string{"tokenCmd", "tokenListCmd", "tokenGetCmd"},
		},
		{
			name:               "internal",
			directives:         "// +fabrica:exposure=internal\n// +fabrica:verbs=list\n",
			wantHandlers:       []string{"func GetTokens("},
			forbidHandlers:     []string{"func GetToken(", "func CreateToken("},
			wantRoute:          "RegisterGeneratedInternalRoutes",
			forbidPublicClient: "func (c *Client) GetToken",
			forbidCLI:          []string{"tokenCmd", "tokenListCmd", "tokenGetCmd"},
			wantHookFile:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := newGeneratedProject(t, "file")
			project.writeResourceSource(t, tt.sourcePrefix+annotatedGeneratedTokenSource(tt.directives))
			if err := os.WriteFile(
				filepath.Join(project.root, "cmd", "server", "main.go"),
				[]byte("package main\n\ntype Config struct{}\n\nfunc main() {}\n"),
				0o644,
			); err != nil {
				t.Fatalf("write server fixture: %v", err)
			}
			serverResult := project.run(
				t,
				"generate-operation-server",
				project.root,
				project.fabricaBin,
				"generate",
				"--force",
				"--debug",
				"--fabrica-source",
				project.repoRoot,
			)
			if serverResult.err != nil {
				t.Fatalf("%s", serverResult.failureMessage())
			}
			clientResult := project.run(
				t,
				"generate-operation-client",
				project.root,
				project.fabricaBin,
				"generate",
				"--client",
				"--force",
				"--debug",
				"--fabrica-source",
				project.repoRoot,
			)
			if clientResult.err != nil {
				t.Fatalf("%s", clientResult.failureMessage())
			}
			if result := project.tidy(t); result.err != nil {
				t.Fatalf("%s", result.failureMessage())
			}
			if result := project.build(t); result.err != nil {
				t.Fatalf("%s", result.failureMessage())
			}

			handlers := readGeneratedArtifact(t, project.root, filepath.Join("cmd", "server", "token_handlers_generated.go"))
			assertContainsAll(t, handlers, tt.wantHandlers...)
			assertContainsNone(t, handlers, tt.forbidHandlers...)
			routes := readGeneratedArtifact(t, project.root, filepath.Join("cmd", "server", "routes_generated.go"))
			if tt.wantRoute != "" && !strings.Contains(routes, tt.wantRoute) {
				t.Errorf("routes missing %q", tt.wantRoute)
			}
			openAPI := readGeneratedArtifact(t, project.root, filepath.Join("cmd", "server", "openapi_generated.go"))
			client := readGeneratedArtifact(t, project.root, filepath.Join("pkg", "client", "client_generated.go"))
			assertContainsAll(t, client, tt.wantClient...)
			clientCLI := readGeneratedArtifact(t, project.root, filepath.Join("cmd", "client", "main.go"))
			assertContainsAll(t, clientCLI, tt.wantCLI...)
			assertContainsNone(t, clientCLI, tt.forbidCLI...)
			if tt.forbidPublicClient != "" {
				assertContainsNone(t, openAPI, "registerTokenPaths", `OperationID = "listTokens"`)
				assertContainsNone(t, client, tt.forbidPublicClient)
			}
			storage := readGeneratedArtifact(t, project.root, filepath.Join("internal", "storage", "storage_generated.go"))
			if !strings.Contains(storage, "Token") {
				t.Error("storage metadata omitted Token")
			}
			registry := readGeneratedArtifact(t, project.root, filepath.Join("pkg", "resources", "register_generated.go"))
			if !strings.Contains(registry, "Token") {
				t.Error("API resource registry omitted Token")
			}
			hookPath := filepath.Join(project.root, "cmd", "server", "token_hooks.go")
			_, hookErr := os.Stat(hookPath)
			if tt.wantHookFile && hookErr != nil {
				t.Errorf("expected handler hook file: %v", hookErr)
			}
			if !tt.wantHookFile && !os.IsNotExist(hookErr) {
				t.Errorf("unexpected handler hook file: %v", hookErr)
			}
		})
	}
}

func TestGeneratedOperationPolicy_invalid_policy_preserves_output_tree(t *testing.T) {
	project := newGeneratedProject(t, "file")
	project.writeResourceSource(t, annotatedGeneratedTokenSource("// +fabrica:verbs=list,get\n"))
	result := project.run(
		t,
		"generate-valid-operation-baseline",
		project.root,
		project.fabricaBin,
		"generate",
		"--force",
		"--debug",
		"--fabrica-source",
		project.repoRoot,
	)
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	before := snapshotAtomicTree(t, project.root)
	project.writeResourceSource(t, annotatedGeneratedTokenSource("// +fabrica:verbs=versionGet\n"))

	result = project.run(
		t,
		"generate-invalid-operation-policy",
		project.root,
		project.fabricaBin,
		"generate",
		"--force",
		"--debug",
		"--fabrica-source",
		project.repoRoot,
	)

	if result.err == nil {
		t.Fatal("invalid version operation generation succeeded")
	}
	if !strings.Contains(result.stdout+result.stderr, "requires resource versioning") {
		t.Fatalf("invalid policy diagnostic missing versioning requirement: %s", result.failureMessage())
	}
	after := snapshotAtomicTree(t, project.root)
	delete(before, filepath.Join("apis", "acceptance.example.io", "v1", "token_types.go"))
	delete(after, filepath.Join("apis", "acceptance.example.io", "v1", "token_types.go"))
	if len(before) != len(after) {
		t.Fatalf("invalid policy changed output file count: before=%d after=%d", len(before), len(after))
	}
	for path, content := range before {
		if string(after[path]) != string(content) {
			t.Errorf("invalid policy changed %s", path)
		}
	}
}

func annotatedGeneratedTokenSource(directives string) string {
	return strings.Replace(generatedUnannotatedTokenSource, "type Token struct {", directives+"type Token struct {", 1)
}

func readGeneratedArtifact(t *testing.T, root, relativePath string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, relativePath))
	if err != nil {
		t.Fatalf("read generated artifact %s: %v", relativePath, err)
	}
	return string(content)
}

func assertContainsAll(t *testing.T, output string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(output, value) {
			t.Errorf("generated output missing %q", value)
		}
	}
}

func assertContainsNone(t *testing.T, output string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(output, value) {
			t.Errorf("generated output unexpectedly contains %q", value)
		}
	}
}

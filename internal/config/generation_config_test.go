// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOpenAPIServerDescription(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml string
		want string
	}{
		"unset": {yaml: "{}\n"},
		"configured": {
			yaml: "generation:\n  openapi_server_description: Production API\n",
			want: "Production API",
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.Generation.OpenAPIServerDescription; got != tc.want {
				t.Fatalf("OpenAPIServerDescription = %q, want %q", got, tc.want)
			}
		})
	}
}

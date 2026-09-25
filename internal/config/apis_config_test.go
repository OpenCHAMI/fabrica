// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"
)

func TestAPIGroupResourcePathUsesDefaultWhenUnset(t *testing.T) {
	group := APIGroup{Resources: []string{"ClusterDefaults"}}

	if got, want := group.ResourcePath("ClusterDefaults"), "/clusterdefaultss"; got != want {
		t.Fatalf("ResourcePath() = %q, want %q", got, want)
	}
}

func TestAPIGroupResourcePathUsesConfiguredPath(t *testing.T) {
	group := APIGroup{
		Resources:     []string{"ClusterDefaults"},
		ResourcePaths: map[string]string{"ClusterDefaults": "/boot/configs"},
	}

	if got, want := group.ResourcePath("ClusterDefaults"), "/boot/configs"; got != want {
		t.Fatalf("ResourcePath() = %q, want %q", got, want)
	}
}

func TestAPIsConfigValidateRejectsInvalidResourcePath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "missing leading slash", path: "boot/configs"},
		{name: "root path", path: "/"},
		{name: "trailing slash", path: "/boot/configs/"},
		{name: "empty segment", path: "/boot//configs"},
		{name: "path parameter", path: "/boot/{uid}"},
		{name: "reserved service status path", path: "/service/status"},
		{name: "reserved health path", path: "/health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAPIsConfigWithResources("ClusterDefaults")
			cfg.Groups[0].ResourcePaths = map[string]string{"ClusterDefaults": tt.path}

			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "invalid path") {
				t.Fatalf("Validate() error = %v, want invalid path error", err)
			}
		})
	}
}

func TestAPIsConfigValidateRejectsResourcePathCollision(t *testing.T) {
	cfg := validAPIsConfigWithResources("ClusterDefaults", "BootConfig")
	cfg.Groups[0].ResourcePaths = map[string]string{"ClusterDefaults": "/bootconfigs"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("Validate() error = %v, want collision error", err)
	}
}

func TestAPIsConfigRevisioningDisabledByDefault(t *testing.T) {
	cfg := validAPIsConfigWithResources("BootConfig")

	revisioning := cfg.Groups[0].RevisioningFor("BootConfig")

	if revisioning.Enabled {
		t.Fatal("RevisioningFor().Enabled = true, want false")
	}
	if got, want := revisioning.BareNameSelector, RevisionSelectorDefault; got != want {
		t.Fatalf("RevisioningFor().BareNameSelector = %q, want %q", got, want)
	}
}

func TestAPIsConfigValidateRevisioningEnabled(t *testing.T) {
	cfg := validAPIsConfigWithResources("BootConfig")
	cfg.Groups[0].Revisioning = map[string]ResourceRevisioning{
		"BootConfig": {
			Enabled:          true,
			BareNameSelector: RevisionSelectorDefault,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if revisioning := cfg.Groups[0].RevisioningFor("BootConfig"); !revisioning.Enabled {
		t.Fatal("RevisioningFor().Enabled = false, want true")
	}
}

func TestAPIsConfigValidateRejectsUnknownRevisioningResource(t *testing.T) {
	cfg := validAPIsConfigWithResources("BootConfig")
	cfg.Groups[0].Revisioning = map[string]ResourceRevisioning{
		"Missing": {Enabled: true},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "revisioning entry Missing") {
		t.Fatalf("Validate() error = %v, want unknown revisioning resource error", err)
	}
}

func TestAPIsConfigValidateRejectsUnsupportedRevisionSelector(t *testing.T) {
	cfg := validAPIsConfigWithResources("BootConfig")
	cfg.Groups[0].Revisioning = map[string]ResourceRevisioning{
		"BootConfig": {
			Enabled:          true,
			BareNameSelector: "newest",
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "unsupported bareNameSelector") {
		t.Fatalf("Validate() error = %v, want unsupported selector error", err)
	}
}

func validAPIsConfigWithResources(resources ...string) *APIsConfig {
	return &APIsConfig{Groups: []APIGroup{{
		Name:           "example.fabrica.dev",
		StorageVersion: "v1",
		Versions:       []string{"v1"},
		Resources:      resources,
	}}}
}

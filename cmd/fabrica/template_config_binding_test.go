// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_InitMain_BindsHyphenatedFlagsToUnderscoreConfigKeys(t *testing.T) {
	mainTmpl := mustReadFile(t, "pkg/codegen/templates/init/main.go.tmpl")

	requiredMarkers := []string{
		`"strings"`,
		`"github.com/spf13/pflag"`,
		"func bindFlagsWithUnderscoreKeys(flags *pflag.FlagSet) error",
		`strings.ReplaceAll(flag.Name, "-", "_")`,
		"viper.BindPFlag(key, flag)",
		"bindFlagsWithUnderscoreKeys(serveCmd.Flags())",
		"bindFlagsWithUnderscoreKeys(rootCmd.PersistentFlags())",
		`viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))`,
	}

	for _, marker := range requiredMarkers {
		if !strings.Contains(mainTmpl, marker) {
			t.Fatalf("init/main.go.tmpl must contain %q", marker)
		}
	}

	forbiddenMarkers := []string{
		"viper.BindPFlags(serveCmd.Flags())",
		"viper.BindPFlags(rootCmd.PersistentFlags())",
	}

	for _, marker := range forbiddenMarkers {
		if strings.Contains(mainTmpl, marker) {
			t.Fatalf("init/main.go.tmpl should not contain %q", marker)
		}
	}
}

func TestTemplate_InitMain_UsesUnderscoreMapstructureKeys(t *testing.T) {
	mainTmpl := mustReadFile(t, "pkg/codegen/templates/init/main.go.tmpl")

	if !strings.Contains(mainTmpl, "DatabaseURL string `mapstructure:\"database_url\"`") {
		t.Fatal("init/main.go.tmpl should use underscore-based database_url mapstructure key")
	}

	if strings.Contains(mainTmpl, "mapstructure:\"database-url\"") {
		t.Fatal("init/main.go.tmpl should not use hyphen-based database-url mapstructure key")
	}

	if !strings.Contains(mainTmpl, `serveCmd.Flags().String("database-url"`) {
		t.Fatal("init/main.go.tmpl should keep the database-url CLI flag hyphenated")
	}
}

func TestTemplate_InitGoMod_IncludesPFlagDependency(t *testing.T) {
	goModTmpl := mustReadFile(t, "pkg/codegen/templates/init/go.mod.tmpl")

	if !strings.Contains(goModTmpl, "github.com/spf13/pflag") {
		t.Fatal("init/go.mod.tmpl should include pflag because generated server imports it directly")
	}
}

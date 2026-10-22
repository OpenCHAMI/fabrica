// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestInitScaffold_DatabaseConfigBehavior(t *testing.T) {
	fixture := t.TempDir()
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, driver := range []string{"sqlite", "sqlite3", "postgres", "mysql"} {
		dir := filepath.Join(fixture, driver)
		if err := createProjectStructure(dir, "tester", &initOptions{
			modulePath: "example.com/configfixture", withStorage: true,
			storageType: "ent", dbDriver: driver,
			apiGroup: "tester.example.com", storageVersion: "v1", apiVersions: []string{"v1"},
		}); err != nil {
			t.Fatal(err)
		}
		if driver == "sqlite" {
			mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err != nil {
				t.Fatal(err)
			}
			write(filepath.Join(fixture, "go.mod"), mod)
		}

		// Execute the actual scaffold's configuration and command declarations.
		// Exclude HTTP/storage orchestration so these tests need no database,
		// authentication service, or generated Ent client.
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(dir, "cmd/server/main.go"), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		var declarations bytes.Buffer
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				if decl.Tok == token.IMPORT {
					continue
				}
			case *ast.FuncDecl:
				switch decl.Name.Name {
				case "DefaultConfig", "bindFlagsWithUnderscoreKeys", "init", "initConfig":
				default:
					continue
				}
			}
			if err := format.Node(&declarations, fset, decl); err != nil {
				t.Fatal(err)
			}
			declarations.WriteString("\n\n")
		}
		// Move only the isolated configuration into the fixture module.
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(dir, "config.go"), []byte(databaseConfigFixtureImports+declarations.String()+
			`func runServer(*cobra.Command, []string) error { return nil }
func newExportCommand() *cobra.Command { return &cobra.Command{Use: "export"} }
func newImportCommand() *cobra.Command { return &cobra.Command{Use: "import"} }
`))
		want := "file:./data.db?cache=shared&_fk=1"
		switch driver {
		case "postgres":
			want = "postgres://localhost/tester?sslmode=disable"
		case "mysql":
			want = "root:@tcp(localhost:3306)/tester?parseTime=true"
		}
		write(filepath.Join(dir, "config_test.go"), []byte(strings.ReplaceAll(databaseConfigFixtureTests, "EXPECTED_DATABASE_URL", strconv.Quote(want))))
	}

	cmd := exec.Command("go", "test", "-mod=mod", "-count=1", "./...")
	cmd.Dir = fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated configuration tests: %v\n%s", err, out)
	}
}

const databaseConfigFixtureImports = `package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
	"os"
	"strings"
)
`

const databaseConfigFixtureTests = `package main

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseConfig(t *testing.T) {
	for _, command := range []string{"root", "serve"} {
		for _, source := range []string{"default", "yaml", "environment", "flag"} {
			t.Run(command+"/"+source, func(t *testing.T) {
				viper.Reset()
				for _, flags := range []*pflag.FlagSet{rootCmd.PersistentFlags(), serveCmd.Flags()} {
					flags.VisitAll(func(flag *pflag.Flag) {
						if err := flag.Value.Set(flag.DefValue); err != nil {
							t.Fatal(err)
						}
						flag.Changed = false
					})
					if err := bindFlagsWithUnderscoreKeys(flags); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("TESTER_DATABASE_URL", "")
				path := filepath.Join(t.TempDir(), "config.yaml")
				args := []string{"--config", path}
				if command == "serve" {
					args = append(args, "serve")
				}
				want := EXPECTED_DATABASE_URL
				if source != "default" {
					want = "file:yaml.db?_fk=1"
					if err := os.WriteFile(path, []byte("database_url: '"+want+"'\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if source == "environment" || source == "flag" {
					want = "file:env.db?_fk=1"
					t.Setenv("TESTER_DATABASE_URL", want)
				}
				if source == "flag" {
					// Database flags belong to serve; root covers defaults, YAML, and env.
					if command == "root" {
						t.Skip("database-url is a serve flag")
					}
					want = "file:flag.db?_fk=1"
					args = append(args, "--database-url", want)
				}
				rootCmd.SetArgs(args)
				if err := rootCmd.Execute(); err != nil {
					t.Fatal(err)
				}
				if config.DatabaseURL != want {
					t.Fatalf("database URL = %q, want %q", config.DatabaseURL, want)
				}
			})
		}
	}
}
`

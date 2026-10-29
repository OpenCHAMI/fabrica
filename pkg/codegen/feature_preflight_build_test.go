// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFeatureLoaderAdoptionUsesTargetFilenames(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	target := build.Default
	otherOS, otherArch := "windows", "amd64"
	if target.GOOS == otherOS {
		otherOS = "linux"
	}
	if target.GOARCH == otherArch {
		otherArch = "arm64"
	}
	const config = "package main\ntype Config struct{GeneratedFeatureConfig}\n"
	const startup = "package main\nfunc start(){loadGeneratedFeatures(nil,nil)}\n"
	for _, tc := range []struct {
		name, configFile, startupFile string
		adopted, invalid              bool
	}{
		{"selected target", "config_" + target.GOOS + ".go", "startup_" + target.GOARCH + ".go", true, false},
		{"inactive OS config", "config_" + otherOS + ".go", "startup.go", false, true},
		{"inactive architecture startup", "config.go", "startup_" + otherArch + ".go", false, true},
		{"both inactive", "config_" + otherOS + ".go", "startup_" + otherArch + ".go", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range map[string]string{"main.go": "package main\n", tc.configFile: config, tc.startupFile: startup} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
					t.Fatal(err)
				}
			}
			adopted, err := UsesFeatureLoader(dir)
			if adopted != tc.adopted || (err != nil) != tc.invalid {
				t.Fatalf("adopted=%v, error=%v", adopted, err)
			}
		})
	}
}

func TestGenerateAllRejectsMalformedBuildFlagsBeforeWrites(t *testing.T) {
	for _, flags := range []string{"-tags", "'-tags=unterminated"} {
		t.Run(flags, func(t *testing.T) {
			g := featureTestGenerator(t)
			root := filepath.Clean(filepath.Join(g.OutputDir, "..", ".."))
			t.Chdir(root)
			if err := g.GenerateAll(); err != nil {
				t.Fatal(err)
			}
			snapshot := func() map[string]string {
				t.Helper()
				files := make(map[string]string)
				err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						return nil
					}
					contents, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					files[path] = string(contents)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				return files
			}
			before := snapshot()
			t.Setenv("GOFLAGS", flags)
			if err := g.GenerateAll(); err == nil || !strings.Contains(err.Error(), "GOFLAGS") {
				t.Fatalf("expected GOFLAGS rejection, got %v", err)
			}
			if !reflect.DeepEqual(before, snapshot()) {
				t.Fatal("rejected build flags changed generated files")
			}
		})
	}
}

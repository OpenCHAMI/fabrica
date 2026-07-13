// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	configpkg "github.com/openchami/fabrica/internal/config"
	"github.com/spf13/cobra"
)

type migrateYAMLTagsOptions struct {
	dryRun bool
	dir    string
}

type yamlTagsMigrationResult struct {
	FilesScanned int
	FilesChanged int
	TagsAdded    int
	ChangedFiles []string
}

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run source migrations for existing Fabrica projects",
		Long: `Run source migrations for existing Fabrica projects.

Migrations update project source files when Fabrica generator behavior changes.
Run migrations before regenerating code with a newer Fabrica version.`,
	}

	cmd.AddCommand(newMigrateYAMLTagsCommand())
	return cmd
}

func newMigrateYAMLTagsCommand() *cobra.Command {
	opts := &migrateYAMLTagsOptions{}
	cmd := &cobra.Command{
		Use:   "yaml-tags",
		Short: "Add YAML struct tags matching existing JSON tags",
		Long: `Add missing yaml struct tags to resource type files.

The migration scans resource definitions from apis.yaml and adds yaml tags that
match existing json tags. Existing yaml tags are preserved, json:"-" fields are
skipped, and all files are gofmt-formatted after changes.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := migrateYAMLTags(opts.dir, opts.dryRun)
			if err != nil {
				return err
			}

			mode := "updated"
			if opts.dryRun {
				mode = "would update"
			}
			fmt.Printf("Scanned %d resource type files; %s %d files; added %d yaml tags.\n", result.FilesScanned, mode, result.FilesChanged, result.TagsAdded)
			for _, file := range result.ChangedFiles {
				fmt.Printf("  %s\n", file)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Report changes without writing files")
	cmd.Flags().StringVar(&opts.dir, "dir", "", "Fabrica project directory (defaults to current directory)")
	return cmd
}

func migrateYAMLTags(projectDir string, dryRun bool) (*yamlTagsMigrationResult, error) {
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	apisConfig, err := configpkg.LoadAPIsConfig(projectDir)
	if err != nil {
		return nil, err
	}

	files, err := resourceTypeFiles(projectDir, apisConfig)
	if err != nil {
		return nil, err
	}

	result := &yamlTagsMigrationResult{FilesScanned: len(files)}
	for _, file := range files {
		changed, tagsAdded, err := addYAMLTagsToFile(file, dryRun)
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		result.FilesChanged++
		result.TagsAdded += tagsAdded
		changedFile := file
		if rel, err := filepath.Rel(projectDir, file); err == nil && !strings.HasPrefix(rel, "..") {
			changedFile = rel
		}
		result.ChangedFiles = append(result.ChangedFiles, filepath.ToSlash(changedFile))
	}

	return result, nil
}

func resourceTypeFiles(projectDir string, apisConfig *configpkg.APIsConfig) ([]string, error) {
	seen := map[string]struct{}{}
	var files []string

	addFile := func(path string) {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		if _, err := os.Stat(clean); err != nil {
			return
		}
		seen[clean] = struct{}{}
		files = append(files, clean)
	}

	for _, group := range apisConfig.Groups {
		for _, version := range group.Versions {
			versionDir := filepath.Join(projectDir, "apis", group.Name, version)
			for _, resource := range group.Resources {
				addFile(filepath.Join(versionDir, strings.ToLower(resource.Name)+"_types.go"))
			}

			entries, err := os.ReadDir(versionDir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("failed to read %s: %w", versionDir, err)
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_types.go") {
					continue
				}
				addFile(filepath.Join(versionDir, entry.Name()))
			}
		}
	}

	sort.Strings(files)
	return files, nil
}

type tagReplacement struct {
	start int
	end   int
	value string
}

func addYAMLTagsToFile(path string, dryRun bool) (bool, int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, 0, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return false, 0, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	var replacements []tagReplacement
	ast.Inspect(node, func(n ast.Node) bool {
		field, ok := n.(*ast.Field)
		if !ok || field.Tag == nil {
			return true
		}

		updated, changed := addYAMLTagToStructTag(field.Tag.Value)
		if !changed {
			return true
		}

		start := fset.Position(field.Tag.Pos()).Offset
		end := fset.Position(field.Tag.End()).Offset
		replacements = append(replacements, tagReplacement{start: start, end: end, value: updated})
		return true
	})

	if len(replacements) == 0 {
		return false, 0, nil
	}
	if dryRun {
		return true, len(replacements), nil
	}

	updated := applyTagReplacements(src, replacements)
	formatted, err := format.Source(updated)
	if err != nil {
		return false, 0, fmt.Errorf("format updated resource file %s: %w", path, err)
	}
	if bytes.Equal(src, formatted) {
		return false, 0, nil
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return false, 0, err
	}
	return true, len(replacements), nil
}

func applyTagReplacements(src []byte, replacements []tagReplacement) []byte {
	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start > replacements[j].start
	})
	updated := append([]byte(nil), src...)
	for _, replacement := range replacements {
		updated = append(updated[:replacement.start], append([]byte(replacement.value), updated[replacement.end:]...)...)
	}
	return updated
}

func addYAMLTagToStructTag(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "`") || !strings.HasSuffix(raw, "`") {
		return raw, false
	}

	parts := strings.Fields(strings.Trim(raw, "`"))
	jsonIndex := -1
	jsonValue := ""
	for i, part := range parts {
		key, quotedValue, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		switch key {
		case "yaml":
			return raw, false
		case "json":
			unquoted, err := strconv.Unquote(quotedValue)
			if err != nil || unquoted == "" || strings.HasPrefix(unquoted, "-") {
				return raw, false
			}
			jsonIndex = i
			jsonValue = unquoted
		}
	}

	if jsonIndex == -1 {
		return raw, false
	}

	yamlPart := fmt.Sprintf("yaml:%q", jsonValue)
	parts = append(parts[:jsonIndex+1], append([]string{yamlPart}, parts[jsonIndex+1:]...)...)
	return "`" + strings.Join(parts, " ") + "`", true
}

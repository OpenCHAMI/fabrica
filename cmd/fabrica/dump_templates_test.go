// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"sort"
	"testing"

	"github.com/openchami/fabrica/pkg/codegen"
)

func TestDumpTemplateKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Skip("debug-only test")
	gen := codegen.NewGenerator("test", "", "")
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	keys := make([]string, 0, len(gen.Templates))
	for k := range gen.Templates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("template key: %s", k)
	}
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

func TestParseResourceAnnotations_operation_policy(t *testing.T) {
	tests := []struct {
		name         string
		directives   string
		wantVerbs    []OperationVerb
		wantExplicit bool
		wantExposure Exposure
		wantError    string
	}{
		{
			name:         "missing directives preserve compatibility defaults",
			wantVerbs:    []OperationVerb{OperationAll},
			wantExposure: ExposureDefault,
		},
		{
			name:         "ordered operation subset",
			directives:   "// +fabrica:verbs=list,get,statusPatch,versionGet\n",
			wantVerbs:    []OperationVerb{OperationList, OperationGet, OperationStatusPatch, OperationVersionGet},
			wantExplicit: true,
			wantExposure: ExposureDefault,
		},
		{
			name:         "private exposure",
			directives:   "// +fabrica:exposure=private\n",
			wantVerbs:    []OperationVerb{OperationAll},
			wantExposure: ExposurePrivate,
		},
		{name: "unknown verb", directives: "// +fabrica:verbs=list,publish\n", wantError: "unknown operation verb"},
		{name: "duplicate verb", directives: "// +fabrica:verbs=get,get\n", wantError: "duplicate operation verb"},
		{name: "empty csv member", directives: "// +fabrica:verbs=list,,get\n", wantError: "empty operation verb"},
		{name: "unknown exposure", directives: "// +fabrica:exposure=external\n", wantError: "unknown exposure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := "package test\n\n" + tt.directives + "// +fabrica:resource\ntype Widget struct{}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "widget.go", source, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			declaration := file.Decls[0].(*ast.GenDecl)
			typeSpec := declaration.Specs[0].(*ast.TypeSpec)

			got, err := ParseResourceAnnotations(typeSpec, declaration.Doc)

			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("ParseResourceAnnotations() error = %v, want containing %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseResourceAnnotations() error = %v", err)
			}
			if !reflect.DeepEqual(got.Verbs, tt.wantVerbs) {
				t.Errorf("Verbs = %#v, want %#v", got.Verbs, tt.wantVerbs)
			}
			if got.VerbsExplicit != tt.wantExplicit {
				t.Errorf("VerbsExplicit = %v, want %v", got.VerbsExplicit, tt.wantExplicit)
			}
			if got.Exposure != tt.wantExposure {
				t.Errorf("Exposure = %q, want %q", got.Exposure, tt.wantExposure)
			}
		})
	}
}

func TestValidate_operation_policy_contradictions(t *testing.T) {
	tests := []struct {
		name      string
		verbs     []OperationVerb
		exposure  Exposure
		wantError string
	}{
		{name: "all alone", verbs: []OperationVerb{OperationAll}},
		{name: "none alone", verbs: []OperationVerb{OperationNone}},
		{name: "all combined", verbs: []OperationVerb{OperationAll, OperationGet}, wantError: "all must appear alone"},
		{name: "none combined", verbs: []OperationVerb{OperationNone, OperationList}, wantError: "none must appear alone"},
		{name: "private explicit none", verbs: []OperationVerb{OperationNone}, exposure: ExposurePrivate},
		{name: "private explicit operation", verbs: []OperationVerb{OperationGet}, exposure: ExposurePrivate, wantError: "private exposure only permits explicit verbs=none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := NewResourceAnnotations()
			annotations.IsResource = true
			annotations.Verbs = tt.verbs
			annotations.VerbsExplicit = true
			if tt.exposure != "" {
				annotations.Exposure = tt.exposure
			}

			err := Validate(annotations)

			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestResolveOperationPolicy(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*ResourceAnnotations)
		versioning bool
		want       OperationPolicy
		wantError  string
	}{
		{
			name: "missing policy preserves current non-versioned surface",
			want: OperationPolicy{
				List: true, Get: true, Create: true, Update: true, Patch: true, Delete: true,
				StatusUpdate: true, StatusPatch: true, Exposure: ExposureDefault,
			},
		},
		{
			name:       "missing policy includes version operations when versioning is enabled",
			versioning: true,
			want: OperationPolicy{
				List: true, Get: true, Create: true, Update: true, Patch: true, Delete: true,
				StatusUpdate: true, StatusPatch: true,
				VersionList: true, VersionGet: true, VersionDelete: true, Exposure: ExposureDefault,
			},
		},
		{
			name: "private without explicit verbs resolves to none",
			configure: func(a *ResourceAnnotations) {
				a.Exposure = ExposurePrivate
			},
			want: OperationPolicy{Exposure: ExposurePrivate},
		},
		{
			name: "read only subset",
			configure: func(a *ResourceAnnotations) {
				a.Verbs = []OperationVerb{OperationList, OperationGet}
				a.VerbsExplicit = true
				a.Exposure = ExposureProtected
			},
			want: OperationPolicy{List: true, Get: true, Exposure: ExposureProtected},
		},
		{
			name: "explicit version operation requires versioning",
			configure: func(a *ResourceAnnotations) {
				a.Verbs = []OperationVerb{OperationVersionGet}
				a.VerbsExplicit = true
			},
			wantError: "requires resource versioning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := NewResourceAnnotations()
			annotations.IsResource = true
			if tt.configure != nil {
				tt.configure(annotations)
			}

			got, err := ResolveOperationPolicy(annotations, tt.versioning)

			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("ResolveOperationPolicy() error = %v, want containing %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveOperationPolicy() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveOperationPolicy() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCloneResourceAnnotations_copies_verbs(t *testing.T) {
	source := NewResourceAnnotations()
	source.Verbs = []OperationVerb{OperationList, OperationGet}

	clone := cloneResourceAnnotations(source)
	clone.Verbs[0] = OperationDelete

	if source.Verbs[0] != OperationList {
		t.Fatalf("source verbs mutated through clone: %#v", source.Verbs)
	}
}

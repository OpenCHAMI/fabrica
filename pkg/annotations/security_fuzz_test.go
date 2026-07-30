// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func FuzzUnknownFabricaDirective(f *testing.F) {
	for _, seed := range [][]byte{[]byte("sensitve"), []byte("storage"), {0x00, 0xff}, []byte("世界")} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 128 {
			t.Skip()
		}
		key := "x" + hex.EncodeToString(raw)
		directive := "+fabrica:" + key + "=value"
		filename := filepath.Join(t.TempDir(), "resource.go")
		source := "package fuzz\n\n// " + directive + "\ntype Record struct{}\n"
		if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := ParseFileAnnotations(filename)
		if got != nil || !errors.Is(err, ErrAnnotationParse) {
			t.Fatalf("reproducer raw=%x directive=%q: ParseFileAnnotations() = %#v, %v", raw, directive, got, err)
		}
		var parseErr *ParseError
		if !errors.As(err, &parseErr) || parseErr.Filename != filename || parseErr.Line <= 0 || parseErr.TypeName != "Record" || parseErr.Directive != directive {
			t.Fatalf("reproducer raw=%x directive=%q: ParseError = %#v", raw, directive, parseErr)
		}
	})
}

func FuzzNonFabricaCommentIgnored(f *testing.F) {
	for _, seed := range [][]byte{[]byte("ordinary comment"), []byte("fabrica:field:sensitive"), {0x00, 0xff}, []byte("+kubebuilder:validation:Required")} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 256 {
			t.Skip()
		}
		comment := hex.EncodeToString(raw)
		filename := filepath.Join(t.TempDir(), "resource.go")
		source := "package fuzz\n\n// ordinary-" + comment + "\ntype Record struct{}\n"
		if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := ParseFileAnnotations(filename)
		if err != nil {
			t.Fatalf("reproducer raw=%x: non-Fabrica comment error = %v", raw, err)
		}
		if got["Record"] == nil || len(got["Record"].RawAnnotations) != 0 {
			t.Fatalf("reproducer raw=%x: annotations = %#v", raw, got)
		}
	})
}

func FuzzMalformedFabricaDirective(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("sensitive"), []byte("storage=hashed"), []byte("cost=12"),
		{0x00}, {0xff}, []byte("世界"),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 128 {
			t.Skip()
		}
		directive := "+fabrica:field::" + hex.EncodeToString(raw)
		filename := filepath.Join(t.TempDir(), "resource.go")
		source := "package fuzz\n\ntype Record struct {\n\t// " + directive + "\n\tValue string\n}\n"
		if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := ParseFileAnnotations(filename)
		if got != nil || !errors.Is(err, ErrAnnotationParse) {
			t.Fatalf("reproducer raw=%x directive=%q: ParseFileAnnotations() = %#v, %v", raw, directive, got, err)
		}
		var parseErr *ParseError
		if !errors.As(err, &parseErr) || parseErr.Filename != filename || parseErr.Line <= 0 || parseErr.TypeName != "Record" || parseErr.FieldName != "Value" || parseErr.Directive != directive {
			t.Fatalf("reproducer raw=%x directive=%q: ParseError = %#v", raw, directive, parseErr)
		}
	})
}

func FuzzFileResolverColdWarm(f *testing.F) {
	seeds := [][2]byte{{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4}, {5, 0}, {6, 1}}
	for _, seed := range seeds {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, typeSeed, directiveSeed byte) {
		types := []string{"string", "bool", "int", "int64", "float64", "time.Time", "[]string"}
		directives := []string{"", "// +fabrica:field:sensitive\n", "// +fabrica:field:index\n", "// +fabrica:field:unique\n", "// +fabrica:field:immutable\n"}
		filename := writeCapabilitySource(t, types[int(typeSeed)%len(types)], directives[int(directiveSeed)%len(directives)])
		t.Cleanup(func() { globalCache.Invalidate(filename) })

		cold, err := ResolveStorageIntent(filename, "Record", DialectPostgreSQL)
		if err != nil {
			t.Fatalf("reproducer type=%d directive=%d: cold error = %v", typeSeed, directiveSeed, err)
		}
		warm, err := ResolveStorageIntent(filename, "Record", DialectPostgreSQL)
		if err != nil {
			t.Fatalf("reproducer type=%d directive=%d: warm error = %v", typeSeed, directiveSeed, err)
		}
		if !reflect.DeepEqual(cold, warm) {
			t.Fatalf("reproducer type=%d directive=%d: cold/warm mismatch", typeSeed, directiveSeed)
		}
		field := warm.Fields[0]
		if cold.Storage != ResourceStorageDedicated || cold.Dialect != DialectPostgreSQL || len(cold.Fields) != 1 || field.Type.Kind == FieldKindUnknown || field.Transform.Kind == TransformUnknown || field.Index == IndexUnknown {
			t.Fatalf("reproducer type=%d directive=%d: capability invariant failed: %#v", typeSeed, directiveSeed, warm)
		}
		warm.Fields[0].GoName = "corrupted"
		later, err := ResolveStorageIntent(filename, "Record", DialectPostgreSQL)
		if err != nil || !reflect.DeepEqual(cold, later) {
			t.Fatalf("reproducer type=%d directive=%d: caller mutation leaked: %#v, %v", typeSeed, directiveSeed, later, err)
		}
	})
}

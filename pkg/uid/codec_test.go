// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package uid_test

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/uid"
	"go.jetify.com/typeid/v2"
	"gopkg.in/yaml.v3"
)

type vector struct {
	Name   string
	TypeID string
	Prefix string
	UUID   string
}

func vectors(t *testing.T, path string) []vector {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []vector
	if err := yaml.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("empty conformance vectors")
	}
	return cases
}

func TestTypeIDConformance(t *testing.T) {
	var codec uid.Codec
	for _, tc := range vectors(t, "testdata/valid.yml") {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := codec.Parse(tc.TypeID)
			if err != nil {
				t.Fatal(err)
			}
			want := uid.ParsedUID{
				Format: uid.FormatTypeID, Prefix: tc.Prefix,
				Suffix: tc.TypeID[len(tc.TypeID)-26:], UUID: tc.UUID,
			}
			if got != want {
				t.Fatalf("Parse(%q) = %+v, want %+v", tc.TypeID, got, want)
			}
			// Verify the pinned encoding implementation against the reverse
			// vectors too, including payloads that intentionally are not v7.
			encoded, err := typeid.FromUUID(got.Prefix, got.UUID)
			if err != nil || encoded.String() != tc.TypeID {
				t.Fatalf("encode vector = %q, %v; want %q", encoded, err, tc.TypeID)
			}
		})
	}
	for _, tc := range vectors(t, "testdata/invalid.yml") {
		t.Run(tc.Name, func(t *testing.T) {
			if got, err := codec.Parse(tc.TypeID); err == nil {
				t.Fatalf("Parse(%q) accepted invalid vector: %+v", tc.TypeID, got)
			}
		})
	}
}

func TestGenerateDeterministic(t *testing.T) {
	id := uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")
	codec := uid.NewCodec(func() ([16]byte, error) { return [16]byte(id), nil })
	got, err := codec.Generate("prefix")
	if err != nil || got != "prefix_01h455vb4pex5vsknk084sn02q" {
		t.Fatalf("Generate() = %q, %v", got, err)
	}
}

func TestGenerateRejectsInvalidSource(t *testing.T) {
	sourceErr := errors.New("entropy unavailable")
	codec := uid.NewCodec(func() ([16]byte, error) { return [16]byte{}, sourceErr })
	if got, err := codec.Generate("dev"); got != "" || !errors.Is(err, sourceErr) {
		t.Fatalf("Generate() = %q, %v; want wrapped source error", got, err)
	}
	for _, value := range []string{
		"00000000-0000-0000-0000-000000000000",
		"01890a5d-ac96-474b-bcce-b302099a8057", // v4
		"01890a5d-ac96-774b-3cce-b302099a8057", // wrong variant
	} {
		t.Run(value, func(t *testing.T) {
			id := uuid.MustParse(value)
			codec := uid.NewCodec(func() ([16]byte, error) { return [16]byte(id), nil })
			if got, err := codec.Generate("dev"); err == nil || got != "" {
				t.Fatalf("Generate() = %q, %v; want UUIDv7 rejection", got, err)
			}
		})
	}
}

func TestPrefixValidation(t *testing.T) {
	for _, prefix := range []string{"", "a", "dev", "pre_fix", "pre__fix", strings.Repeat("a", 63)} {
		if err := uid.ValidatePrefix(prefix); err != nil {
			t.Errorf("ValidatePrefix(%q): %v", prefix, err)
		}
		// The default generator must handle every supported prefix too.
		value, err := uid.NewCodec(nil).Generate(prefix)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := (uid.Codec{}).Parse(value)
		if err != nil || parsed.Prefix != prefix {
			t.Fatalf("round trip prefix %q = %+v, %v", prefix, parsed, err)
		}
	}
	codec := uid.NewCodec(func() ([16]byte, error) {
		t.Fatal("generator called before prefix validation")
		return [16]byte{}, nil
	})
	for _, prefix := range []string{"Dev", "dev1", "1dev", "_dev", "dev_", "dev-x", "dévice", "dev x", strings.Repeat("a", 64)} {
		if err := uid.ValidatePrefix(prefix); err == nil {
			t.Errorf("ValidatePrefix(%q) accepted invalid prefix", prefix)
		}
		if got, err := codec.Generate(prefix); err == nil || got != "" {
			t.Errorf("Generate(%q) = %q, %v", prefix, got, err)
		}
	}
}

func TestLegacyCompatibility(t *testing.T) {
	var codec uid.Codec
	// Preserve even historical permissive cases until a deliberate public
	// validation change is made. Do not reinterpret or normalize stored IDs.
	for _, value := range []string{"dev-1a2b3c4d", "dev-ABCDEF", "disk1-aabbccddeeff", "Odd_Prefix-aa", "-", "dev-", "-aa"} {
		prefix, suffix, err := parseLegacyUID(value)
		if err != nil {
			t.Fatal(err)
		}
		got, err := codec.Parse(value)
		want := uid.ParsedUID{Format: uid.FormatLegacy, Prefix: prefix, Suffix: suffix}
		if err != nil || got != want {
			t.Fatalf("Parse(%q) = %+v, %v; want %+v", value, got, err, want)
		}
	}
	for _, value := range []string{"dev-a", "dev-zz", "dev-aa-bb"} {
		if _, err := codec.Parse(value); err == nil {
			t.Errorf("Parse(%q) accepted invalid legacy UID", value)
		}
	}
}

func TestResourceGenerationUsesTypeID(t *testing.T) {
	value, err := resource.GenerateUID("dev")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := (uid.Codec{}).Parse(value)
	if err != nil || parsed.Format != uid.FormatTypeID || parsed.Prefix != "dev" {
		t.Fatalf("resource generation must use TypeID: %q, %v", value, err)
	}
	if !resource.IsValidUID("dev_01h455vb4pex5vsknk084sn02q") {
		t.Fatal("resource validation must accept TypeID during dual parsing rollout")
	}
}

func TestGenerateConcurrent(t *testing.T) {
	var codec uid.Codec
	const workers, perWorker = 16, 128
	values := make(chan string, workers*perWorker)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range perWorker {
				value, err := codec.Generate("dev")
				if err != nil {
					t.Error(err)
					return
				}
				values <- value
			}
		})
	}
	wg.Wait()
	close(values)
	seen := make(map[string]bool)
	for value := range values {
		if seen[value] {
			t.Fatalf("duplicate generated UID: %q", value)
		}
		seen[value] = true
		parsed, err := codec.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		id, err := uuid.Parse(parsed.UUID)
		if err != nil || id.Version() != 7 || id.Variant() != uuid.RFC4122 || parsed.Prefix != "dev" {
			t.Fatalf("invalid generated TypeID: %+v, %v", parsed, err)
		}
		encoded, err := typeid.FromUUID(parsed.Prefix, parsed.UUID)
		if err != nil || encoded.String() != value {
			t.Fatalf("round trip = %q, %v; want %q", encoded, err, value)
		}
	}
	if len(seen) != workers*perWorker {
		t.Fatalf("generated %d IDs, want %d", len(seen), workers*perWorker)
	}
}

func FuzzParse(f *testing.F) {
	for _, value := range []string{"dev-1a2b3c4d", "dev_01h455vb4pex5vsknk084sn02q", "pre_fix_00000000000000000000000000", "", "-", "dev-zz", "dev_8zzzzzzzzzzzzzzzzzzzzzzzzz"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		parsed, err := (uid.Codec{}).Parse(value)
		if err != nil {
			return
		}
		switch parsed.Format {
		case uid.FormatLegacy:
			prefix, suffix, err := parseLegacyUID(value)
			if err != nil || prefix != parsed.Prefix || suffix != parsed.Suffix || parsed.UUID != "" {
				t.Fatalf("legacy parser mismatch: %+v, %v", parsed, err)
			}
		case uid.FormatTypeID:
			encoded, err := typeid.FromUUID(parsed.Prefix, parsed.UUID)
			if err != nil || encoded.String() != value {
				t.Fatalf("TypeID round trip mismatch: %+v, %v", parsed, err)
			}
		default:
			t.Fatalf("unknown format: %+v", parsed)
		}
	})
}

// Freeze the pre-migration legacy parser as an independent compatibility oracle.
func parseLegacyUID(value string) (string, string, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return "", "", errors.New("expected prefix-hex")
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", "", err
	}
	return parts[0], parts[1], nil
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package resource

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.jetify.com/typeid/v2"
)

func TestParseUIDFormats(t *testing.T) {
	for _, tc := range []struct {
		value, prefix, suffix string
	}{
		{"dev-1a2b3c4d", "dev", "1a2b3c4d"},
		{"disk1-ABCDEF", "disk1", "ABCDEF"},
		{"Odd_Prefix-aa", "Odd_Prefix", "aa"},
		{"-", "", ""},
		{"dev-", "dev", ""},
		{"-aa", "", "aa"},
		{"dev_01h455vb4pex5vsknk084sn02q", "dev", "01h455vb4pex5vsknk084sn02q"},
		{"pre_fix_00000000000000000000000000", "pre_fix", "00000000000000000000000000"},
		{"00000000000000000000000000", "", "00000000000000000000000000"},
		// Parsing accepts spec-valid payloads even when they are not UUIDv7.
		{"dev_0123456789abcdefghjkmnpqrs", "dev", "0123456789abcdefghjkmnpqrs"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			prefix, suffix, err := ParseUID(tc.value)
			if err != nil || prefix != tc.prefix || suffix != tc.suffix {
				t.Fatalf("ParseUID(%q) = %q, %q, %v", tc.value, prefix, suffix, err)
			}
			if !IsValidUID(tc.value) {
				t.Fatalf("IsValidUID(%q) = false", tc.value)
			}
		})
	}
	for _, value := range []string{
		"", "dev", "dev-abc", "dev-zz", "dev-aa-bb",
		"dev_", "_00000000000000000000000000", "dev__00000000000000000000000000",
		"Dev_01h455vb4pex5vsknk084sn02q", "dev1_01h455vb4pex5vsknk084sn02q",
		"dev_8zzzzzzzzzzzzzzzzzzzzzzzzz", "dev_01H455VB4PEX5VSKNK084SN02Q",
		"dev_0000000000000000000000000i",
	} {
		t.Run("invalid/"+value, func(t *testing.T) {
			prefix, suffix, err := ParseUID(value)
			if err == nil || prefix != "" || suffix != "" || IsValidUID(value) {
				t.Fatalf("invalid UID %q accepted: %q, %q, %v", value, prefix, suffix, err)
			}
			if _, err := GetResourceTypeFromUID(value); err == nil {
				t.Fatalf("lookup accepted invalid UID %q", value)
			}
		})
	}
}

func TestResourceTypeLookupBothFormats(t *testing.T) {
	// Registry tests run serially and restore the exact prior registrations.
	resourcePrefixesMutex.Lock()
	saved := resourcePrefixes
	resourcePrefixes = make(map[string]string)
	resourcePrefixesMutex.Unlock()
	t.Cleanup(func() {
		resourcePrefixesMutex.Lock()
		resourcePrefixes = saved
		resourcePrefixesMutex.Unlock()
	})
	RegisterResourcePrefix("Device", "dev")
	RegisterResourcePrefix("DiskV1", "disk1") // numeric registrations still work
	for _, value := range []string{"dev-1a2b3c4d", "dev_01h455vb4pex5vsknk084sn02q", "dev_00000000000000000000000000"} {
		kind, err := GetResourceTypeFromUID(value)
		if err != nil || kind != "Device" {
			t.Fatalf("GetResourceTypeFromUID(%q) = %q, %v", value, kind, err)
		}
	}
	RegisterResourcePrefix("GroupName", "group_name")
	for _, kind := range []string{"Device", "GroupName"} {
		value, err := GenerateUIDForResource(kind)
		if err != nil {
			t.Fatal(err)
		}
		got, err := GetResourceTypeFromUID(value)
		id, parseErr := typeid.Parse(value)
		if err != nil || got != kind || parseErr != nil || uuid.MustParse(id.UUID()).Version() != 7 {
			t.Fatalf("TypeID generation/lookup for %q = %q, %q, %v", kind, value, got, err)
		}
	}
	if value, err := GenerateUIDForResource("DiskV1"); err == nil || value != "" {
		t.Fatalf("numeric prefix must reject new generation: %q, %v", value, err)
	}
	if kind, err := GetResourceTypeFromUID("disk1-abcdef"); err != nil || kind != "DiskV1" {
		t.Fatalf("numeric legacy lookup: %q, %v", kind, err)
	}
	for _, value := range []string{"missing-aa", "missing_01h455vb4pex5vsknk084sn02q", "pre_fix_00000000000000000000000000", "00000000000000000000000000"} {
		if !IsValidUID(value) {
			t.Fatalf("unregistered UID %q should parse", value)
		}
		if _, err := GetResourceTypeFromUID(value); err == nil {
			t.Fatalf("lookup accepted unregistered UID %q", value)
		}
	}
}

func FuzzParseUID(f *testing.F) {
	for _, value := range []string{"dev-1a2b3c4d", "disk1-ABCDEF", "-", "Odd_Prefix-", "dev-zz", "dev-aa-bb", "dev_01h455vb4pex5vsknk084sn02q", "pre_fix_00000000000000000000000000", "00000000000000000000000000", "dev_8zzzzzzzzzzzzzzzzzzzzzzzzz", ""} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		prefix, suffix, err := ParseUID(value)
		var wantPrefix, wantSuffix string
		var valid bool
		if strings.Contains(value, "-") {
			// Independent oracle frozen from the legacy implementation.
			parts := strings.Split(value, "-")
			if len(parts) == 2 {
				_, hexErr := hex.DecodeString(parts[1])
				valid = hexErr == nil
				if valid {
					wantPrefix, wantSuffix = parts[0], parts[1]
				}
			}
		} else {
			id, parseErr := typeid.Parse(value)
			valid = parseErr == nil
			if valid {
				wantPrefix, wantSuffix = id.Prefix(), id.Suffix()
			}
		}
		if (err == nil) != valid || prefix != wantPrefix || suffix != wantSuffix || IsValidUID(value) != valid {
			t.Fatalf("ParseUID(%q) = %q, %q, %v; want %q, %q, valid=%t", value, prefix, suffix, err, wantPrefix, wantSuffix, valid)
		}
	})
}

func TestGenerateUIDRejectsIncompatiblePrefixes(t *testing.T) {
	for _, prefix := range []string{"", "disk1", "DEV", "_dev", "dev_", "dev-x", strings.Repeat("a", 64)} {
		if value, err := GenerateUID(prefix); err == nil || value != "" {
			t.Fatalf("GenerateUID(%q) = %q, %v", prefix, value, err)
		}
	}
}

func TestGenerateUIDWithLengthRemainsExplicitLegacy(t *testing.T) {
	value, err := GenerateUIDWithLength("disk1", 12)
	if err != nil {
		t.Fatal(err)
	}
	prefix, suffix, err := ParseUID(value)
	if err != nil || prefix != "disk1" || len(suffix) != 12 || !strings.Contains(value, "-") {
		t.Fatalf("legacy helper = %q, %v", value, err)
	}
}

func TestRegisterResourcePrefixRejectsMalformedTypeIDPrefix(t *testing.T) {
	for _, prefix := range []string{"", "_dev", "dev_", "dev_1", "DEV", "dev-x"} {
		t.Run(prefix, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("registration accepted invalid prefix")
				}
			}()
			RegisterResourcePrefix("InvalidPrefix", prefix)
		})
	}
}

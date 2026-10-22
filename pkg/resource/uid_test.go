// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package resource

import (
	"encoding/hex"
	"strings"
	"testing"

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
	for _, kind := range []string{"Device", "DiskV1"} {
		value, err := GenerateUIDForResource(kind)
		if err != nil {
			t.Fatal(err)
		}
		got, err := GetResourceTypeFromUID(value)
		_, suffix, parseErr := ParseUID(value)
		if err != nil || got != kind || !strings.Contains(value, "-") || parseErr != nil || len(suffix) != 8 {
			t.Fatalf("legacy generation/lookup for %q = %q, %q, %v", kind, value, got, err)
		}
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

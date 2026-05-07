// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import "testing"

func TestPluralizeResourceName(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "Device", mode: "smart", want: "devices"},
		{name: "Policy", mode: "smart", want: "policies"},
		{name: "Class", mode: "smart", want: "classes"},
		{name: "ClusterDefaults", mode: "smart", want: "clusterdefaults"},
		{name: "Bus", mode: "legacy", want: "buss"},
	}

	for _, tc := range tests {
		if got := pluralizeResourceName(tc.name, tc.mode); got != tc.want {
			t.Fatalf("pluralizeResourceName(%q, %q) = %q, want %q", tc.name, tc.mode, got, tc.want)
		}
	}
}

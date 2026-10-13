// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package resourcepath

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name:    "reserved metrics endpoint",
			path:    "/metrics",
			wantErr: `path "/metrics" is reserved for a service endpoint`,
		},
		{
			name: "metrics extra remains valid",
			path: "/metrics-extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.path)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate(%q) returned error %v, want nil", tt.path, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate(%q) returned nil, want %q", tt.path, tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("Validate(%q) error = %q, want %q", tt.path, err.Error(), tt.wantErr)
			}
		})
	}
}

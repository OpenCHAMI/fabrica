// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package validation

import "testing"

type requiredPresenceRequest struct {
	Spec    struct{} `json:"spec" validate:"required"`
	present bool
}

func (r *requiredPresenceRequest) RequiredFieldPresent(name string) (bool, bool) {
	return r.present, name == "spec"
}

func TestValidateRequiredFieldsOnlyChecksImmediateEnvelope(t *testing.T) {
	type spec struct {
		Description string `validate:"required"`
		Marker      int
	}
	type request struct {
		Name string `json:"name" validate:"required"`
		Spec spec   `json:"spec" validate:"required"`
	}

	if err := ValidateRequiredFields(&request{}); err == nil {
		t.Fatal("ValidateRequiredFields() accepted missing immediate required fields")
	}
	if err := ValidateRequiredFields(&request{Name: "node", Spec: spec{Marker: 1}}); err != nil {
		t.Fatalf("ValidateRequiredFields() rejected nested optional-policy field: %v", err)
	}
	if err := ValidateRequiredFields(&request{Spec: spec{Marker: 1}}, "spec"); err != nil {
		t.Fatalf("ValidateRequiredFields() rejected an unselected path-supplied identifier: %v", err)
	}
	if err := ValidateRequiredFields(&requiredPresenceRequest{present: true}); err != nil {
		t.Fatalf("ValidateRequiredFields() rejected a present zero-value envelope: %v", err)
	}
	if err := ValidateRequiredFields(&requiredPresenceRequest{}); err == nil {
		t.Fatal("ValidateRequiredFields() accepted an absent envelope")
	}
}

func TestValidateRequiredFieldsRejectsInvalidRequests(t *testing.T) {
	var nilRequest *requiredPresenceRequest
	for _, value := range []any{nil, nilRequest, 1, "invalid", []string{"invalid"}} {
		if err := ValidateRequiredFields(value); err == nil {
			t.Fatalf("accepted invalid request %T", value)
		}
	}
}

// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package validation

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestResolveMode(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, mode := range []string{"", "strict", "warn", "disabled", "invalid"} {
			got, err := ResolveMode(enabled, mode)
			if mode == "invalid" {
				if err == nil {
					t.Fatal("invalid mode accepted")
				}
				continue
			}
			want := Mode(mode)
			if mode == "" {
				want = Strict
			}
			if !enabled || mode == "disabled" {
				want = Disabled
			}
			if err != nil || got != want {
				t.Fatalf("ResolveMode(%t, %q) = %q, %v; want %q", enabled, mode, got, err, want)
			}
		}
	}
}

type policySpec struct {
	Value string `json:"value" validate:"required,max=5"`
}
type policyStatus struct {
	Phase string `json:"phase" validate:"oneof=Ready Pending"`
}
type policyRequest struct {
	Identity string       `json:"identity" validate:"required"`
	Spec     *policySpec  `json:"spec" validate:"required"`
	Status   policyStatus `json:"status"`
	calls    int
}

func (p *policyRequest) Validate(context.Context) error {
	p.calls++
	return errors.New("custom rejection")
}

func TestRequestPolicyPreservesMandatoryChecks(t *testing.T) {
	for _, mode := range []Mode{Strict, Warn, Disabled} {
		t.Run(string(mode), func(t *testing.T) {
			for _, value := range []*policyRequest{
				{Spec: &policySpec{Value: "valid"}},
				{Identity: "required"},
			} {
				if err := ValidateRequestWithContext(context.Background(), value, mode); err == nil {
					t.Fatalf("mandatory fields accepted in %s: %+v", mode, value)
				}
				if value.calls != 0 {
					t.Fatal("custom hook ran after mandatory rejection")
				}
			}
			value := &policyRequest{Identity: "required", Spec: &policySpec{Value: "too long"}, Status: policyStatus{Phase: "Broken"}}
			var logs bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(previous) })
			err := ValidateRequestWithContext(context.Background(), value, mode)
			if (err != nil) != (mode == Strict) {
				t.Fatalf("mode %s: %v", mode, err)
			}
			wantCalls := 0
			if mode == Warn {
				wantCalls = 1
			}
			if value.calls != wantCalls {
				t.Fatalf("custom calls=%d, want %d", value.calls, wantCalls)
			}
			if mode == Warn && (!strings.Contains(logs.String(), "validation failed") || !strings.Contains(logs.String(), "custom rejection")) {
				t.Fatalf("warn log: %s", logs.String())
			}
			if mode == Disabled && logs.Len() != 0 {
				t.Fatalf("disabled logged: %s", logs.String())
			}
			// Standalone callers retain their original strict behavior.
			if err := ValidateResource(value); err == nil {
				t.Fatal("standalone tags became optional")
			}
			if err := ValidateWithContext(context.Background(), value); err == nil {
				t.Fatal("standalone custom validation became optional")
			}
		})
	}
}

func TestOptionalStatusAndCustomPolicy(t *testing.T) {
	for _, mode := range []Mode{Strict, Warn, Disabled} {
		if err := ValidateWithContextMode(context.Background(), &policyStatus{Phase: "Broken"}, mode); (err != nil) != (mode == Strict) {
			t.Fatalf("status mode %s: %v", mode, err)
		}
		p := &policyRequest{Identity: "required", Spec: &policySpec{Value: "valid"}, Status: policyStatus{Phase: "Ready"}}
		err := ValidateRequestWithContext(context.Background(), p, mode)
		if (err != nil) != (mode == Strict) {
			t.Fatalf("custom mode %s: %v", mode, err)
		}
		want := 1
		if mode == Disabled {
			want = 0
		}
		if p.calls != want {
			t.Fatalf("custom calls=%d, want %d", p.calls, want)
		}
	}
	if err := ValidateWithContextMode(context.Background(), &policyStatus{Phase: "Ready"}, Mode("bad")); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

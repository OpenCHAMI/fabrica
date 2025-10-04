// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
)

// Test structures

type TestResource struct {
	Name        string            `json:"name" validate:"required,k8sname"`
	Description string            `json:"description" validate:"max=100"`
	Labels      map[string]string `json:"labels" validate:"dive,keys,labelkey,endkeys,labelvalue"`
	Email       string            `json:"email" validate:"omitempty,email"`
}

type TestResourceWithCustomValidation struct {
	Name string `json:"name" validate:"required"`
}

func (tr *TestResourceWithCustomValidation) Validate(_ context.Context) error {
	if tr.Name == "forbidden" {
		return errors.New("name 'forbidden' is not allowed")
	}
	return nil
}

// Test ValidateResource

func TestValidateResource_Valid(t *testing.T) {
	resource := TestResource{
		Name:        "test-resource",
		Description: "A test resource",
		Labels: map[string]string{
			"app":     "test",
			"version": "1.0",
		},
		Email: "test@example.com",
	}

	err := ValidateResource(&resource)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestValidateResource_RequiredField(t *testing.T) {
	resource := TestResource{
		// Missing required Name field
		Description: "A test resource",
	}

	err := ValidateResource(&resource)
	if err == nil {
		t.Error("Expected validation error for missing required field")
	}

	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Errorf("Expected ValidationErrors, got: %T", err)
	}

	if len(validationErrs.Errors) != 1 {
		t.Errorf("Expected 1 error, got: %d", len(validationErrs.Errors))
	}

	if validationErrs.Errors[0].Field != "name" {
		t.Errorf("Expected error for 'name', got: %s", validationErrs.Errors[0].Field)
	}
}

func TestValidateResource_MaxLength(t *testing.T) {
	resource := TestResource{
		Name:        "test-resource",
		Description: string(make([]byte, 101)), // Exceeds max=100
	}

	err := ValidateResource(&resource)
	if err == nil {
		t.Error("Expected validation error for exceeding max length")
	}
}

func TestValidateResource_InvalidEmail(t *testing.T) {
	resource := TestResource{
		Name:  "test-resource",
		Email: "not-an-email",
	}

	err := ValidateResource(&resource)
	if err == nil {
		t.Error("Expected validation error for invalid email")
	}
}

// Test ValidateWithContext

func TestValidateWithContext_Valid(t *testing.T) {
	resource := TestResourceWithCustomValidation{
		Name: "valid-name",
	}

	err := ValidateWithContext(context.Background(), &resource)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestValidateWithContext_CustomValidationFails(t *testing.T) {
	resource := TestResourceWithCustomValidation{
		Name: "forbidden",
	}

	err := ValidateWithContext(context.Background(), &resource)
	if err == nil {
		t.Error("Expected custom validation error")
	}

	if err.Error() != "name 'forbidden' is not allowed" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// Test K8s name validation

func TestValidateK8sName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{"valid simple", "test", true},
		{"valid with hyphen", "test-resource", true},
		{"valid with dot", "test.resource", true},
		{"valid with number", "test123", true},
		{"valid long", string(make([]byte, 253)), true},
		{"invalid empty", "", false},
		{"invalid too long", string(make([]byte, 254)), false},
		{"invalid uppercase", "TestResource", false},
		{"invalid starts with hyphen", "-test", false},
		{"invalid ends with hyphen", "test-", false},
		{"invalid starts with dot", ".test", false},
		{"invalid ends with dot", "test.", false},
		{"invalid underscore", "test_resource", false},
		{"invalid special char", "test@resource", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the actual test value
			name := tt.value
			// Fill with 'x' for length tests
			if len(tt.value) == 253 || len(tt.value) == 254 {
				name = ""
				targetLen := len(tt.value)
				for i := 0; i < targetLen; i++ {
					if i == 0 || i == targetLen-1 {
						name += "a" // Start and end with alphanumeric
					} else {
						name += "x"
					}
				}
			}

			resource := TestResource{
				Name: name,
			}

			err := ValidateResource(&resource)
			if tt.valid && err != nil {
				t.Errorf("Expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("Expected validation error, got nil")
			}
		})
	}
}

// Test label key validation

func TestValidateLabelKey(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"valid simple", "app", true},
		{"valid with hyphen", "app-name", true},
		{"valid with underscore", "app_name", true},
		{"valid with dot", "app.name", true},
		{"valid with prefix", "example.com/app", true},
		{"valid long prefix", "long.subdomain.example.com/app", true},
		{"invalid empty", "", false},
		{"invalid too long name", string(make([]byte, 64)), false},
		{"invalid starts with hyphen", "-app", false},
		{"invalid ends with hyphen", "app-", false},
		{"invalid uppercase in prefix", "Example.com/app", false},
		{"invalid multiple slashes", "example.com/app/name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For length test, fill with 'x'
			key := tt.key
			if len(key) == 64 {
				key = ""
				for i := 0; i < 64; i++ {
					key += "x"
				}
			}

			type LabelResource struct {
				Key string `json:"key" validate:"labelkey"`
			}

			resource := LabelResource{Key: key}
			err := ValidateResource(&resource)

			if tt.valid && err != nil {
				t.Errorf("Expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("Expected validation error, got nil")
			}
		})
	}
}

// Test label value validation

func TestValidateLabelValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{"valid simple", "test", true},
		{"valid with hyphen", "test-value", true},
		{"valid with underscore", "test_value", true},
		{"valid with dot", "test.value", true},
		{"valid empty", "", true},
		{"valid 63 chars", string(make([]byte, 63)), true},
		{"invalid 64 chars", string(make([]byte, 64)), false},
		{"invalid starts with hyphen", "-test", false},
		{"invalid ends with hyphen", "test-", false},
		{"invalid starts with underscore", "_test", false},
		{"invalid special char", "test@value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.value
			// Fill with 'x' for length tests
			if len(value) == 63 || len(value) == 64 {
				value = ""
				for i := 0; i < len(tt.value); i++ {
					if i == 0 || i == len(tt.value)-1 {
						value += "a" // Start and end with alphanumeric
					} else {
						value += "x"
					}
				}
			}

			type ValueResource struct {
				Value string `json:"value" validate:"labelvalue"`
			}

			resource := ValueResource{Value: value}
			err := ValidateResource(&resource)

			if tt.valid && err != nil {
				t.Errorf("Expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("Expected validation error, got nil")
			}
		})
	}
}

// Test DNS subdomain validation

func TestValidateDNSSubdomain(t *testing.T) {
	tests := []struct {
		name      string
		subdomain string
		valid     bool
	}{
		{"valid simple", "example", true},
		{"valid with subdomain", "api.example.com", true},
		{"valid with many levels", "a.b.c.d.example.com", true},
		{"valid with hyphen", "my-api.example.com", true},
		{"valid 253 chars", string(make([]byte, 253)), true},
		{"invalid empty", "", false},
		{"invalid 254 chars", string(make([]byte, 254)), false},
		{"invalid uppercase", "Example.com", false},
		{"invalid starts with hyphen", "-example.com", false},
		{"invalid ends with hyphen", "example-.com", false},
		{"invalid double dot", "example..com", false},
		{"invalid ends with dot", "example.com.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subdomain := tt.subdomain
			// Fill with valid pattern for length tests
			if len(subdomain) == 253 || len(subdomain) == 254 {
				subdomain = ""
				for len(subdomain) < len(tt.subdomain) {
					if len(subdomain)+10 <= len(tt.subdomain) {
						subdomain += "example123"
					} else {
						subdomain += "a"
					}
					if len(subdomain) < len(tt.subdomain) {
						subdomain += "."
					}
				}
				// Ensure it doesn't end with a dot
				subdomain = subdomain[:len(tt.subdomain)]
				if subdomain[len(subdomain)-1] == '.' {
					subdomain = subdomain[:len(subdomain)-1] + "a"
				}
			}

			type DNSResource struct {
				Domain string `json:"domain" validate:"dnssubdomain"`
			}

			resource := DNSResource{Domain: subdomain}
			err := ValidateResource(&resource)

			if tt.valid && err != nil {
				t.Errorf("Expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("Expected validation error, got nil")
			}
		})
	}
}

// Test DNS label validation

func TestValidateDNSLabel(t *testing.T) {
	tests := []struct {
		name  string
		label string
		valid bool
	}{
		{"valid simple", "example", true},
		{"valid with hyphen", "my-label", true},
		{"valid with number", "label123", true},
		{"valid 63 chars", string(make([]byte, 63)), true},
		{"invalid empty", "", false},
		{"invalid 64 chars", string(make([]byte, 64)), false},
		{"invalid uppercase", "Example", false},
		{"invalid starts with hyphen", "-example", false},
		{"invalid ends with hyphen", "example-", false},
		{"invalid dot", "my.label", false},
		{"invalid underscore", "my_label", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label := tt.label
			// Fill with 'x' for length tests
			if len(label) == 63 || len(label) == 64 {
				label = "a"
				for i := 1; i < len(tt.label)-1; i++ {
					label += "x"
				}
				if len(tt.label) > 1 {
					label += "a"
				}
			}

			type LabelResource struct {
				Label string `json:"label" validate:"dnslabel"`
			}

			resource := LabelResource{Label: label}
			err := ValidateResource(&resource)

			if tt.valid && err != nil {
				t.Errorf("Expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("Expected validation error, got nil")
			}
		})
	}
}

// Test error formatting

func TestValidationErrors_Error(t *testing.T) {
	ve := ValidationErrors{
		Errors: []FieldError{
			{Field: "name", Tag: "required", Message: "name is required"},
			{Field: "email", Tag: "email", Message: "email must be a valid email address"},
		},
	}

	expected := "name is required; email must be a valid email address"
	if ve.Error() != expected {
		t.Errorf("Expected error message '%s', got: '%s'", expected, ve.Error())
	}
}

func TestFieldError_Error(t *testing.T) {
	fe := FieldError{
		Field:   "name",
		Tag:     "required",
		Message: "name is required",
	}

	expected := "name is required"
	if fe.Error() != expected {
		t.Errorf("Expected error message '%s', got: '%s'", expected, fe.Error())
	}
}

// Test custom validator registration

func TestRegisterCustomValidator(t *testing.T) {
	// Register a custom validator that checks if a string equals "test"
	err := RegisterCustomValidator("customtest", func(fl validator.FieldLevel) bool {
		return fl.Field().String() == "test"
	})

	if err != nil {
		t.Fatalf("Failed to register custom validator: %v", err)
	}

	type CustomResource struct {
		Value string `json:"value" validate:"customtest"`
	}

	// Test valid value
	validResource := CustomResource{Value: "test"}
	err = ValidateResource(&validResource)
	if err != nil {
		t.Errorf("Expected no error for valid custom validation, got: %v", err)
	}

	// Test invalid value
	invalidResource := CustomResource{Value: "nottest"}
	err = ValidateResource(&invalidResource)
	if err == nil {
		t.Error("Expected validation error for invalid custom validation")
	}
}

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package validation provides comprehensive validation for Fabrica resources.
//
// This package combines go-playground/validator for struct-tag based validation
// with custom validators for Kubernetes-style resource patterns.
//
// Usage:
//
//	// In your resource definition
//	type Device struct {
//	    Resource
//	    Spec DeviceSpec `json:"spec" validate:"required"`
//	}
//
//	type DeviceSpec struct {
//	    Name string `json:"name" validate:"required,k8sname"`
//	}
//
//	// In your handler
//	if err := validation.ValidateResource(&device); err != nil {
//	    return err
//	}
package validation

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Global validator instance
var validate *validator.Validate

// ValidationErrors wraps multiple validation errors
//
//nolint:revive // "ValidationErrors" name is intentional; "Errors" alone would be ambiguous
type ValidationErrors struct {
	Errors []FieldError `json:"errors"`
}

func (ve ValidationErrors) Error() string {
	var msgs []string
	for _, err := range ve.Errors {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// FieldError represents a single field validation error
type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

func (fe FieldError) Error() string {
	return fe.Message
}

func init() {
	validate = validator.New()

	// Register function to get JSON field names instead of struct field names
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Register custom validators
	_ = validate.RegisterValidation("k8sname", validateK8sName)
	_ = validate.RegisterValidation("labelkey", validateLabelKey)
	_ = validate.RegisterValidation("labelvalue", validateLabelValue)
	_ = validate.RegisterValidation("dnssubdomain", validateDNSSubdomain)
	_ = validate.RegisterValidation("dnslabel", validateDNSLabel)
}

// ValidateResource validates a resource using struct tags
func ValidateResource(resource interface{}) error {
	if err := validate.Struct(resource); err != nil {
		if validationErrs, ok := err.(validator.ValidationErrors); ok {
			return formatValidationErrors(validationErrs)
		}
		return err
	}
	return nil
}

// ValidateWithContext validates a resource with context-aware custom validation
func ValidateWithContext(ctx context.Context, resource interface{}) error {
	// First do struct validation
	if err := ValidateResource(resource); err != nil {
		return err
	}

	// Then run custom validators if implemented
	if customValidator, ok := resource.(CustomValidator); ok {
		if err := customValidator.Validate(ctx); err != nil {
			return err
		}
	}

	return nil
}

// CustomValidator interface allows resources to implement custom validation logic
type CustomValidator interface {
	Validate(ctx context.Context) error
}

// formatValidationErrors converts validator errors to user-friendly messages
func formatValidationErrors(errs validator.ValidationErrors) error {
	var fieldErrors []FieldError

	for _, err := range errs {
		fieldErrors = append(fieldErrors, FieldError{
			Field:   err.Field(),
			Tag:     err.Tag(),
			Value:   fmt.Sprintf("%v", err.Value()),
			Message: getErrorMessage(err),
		})
	}

	return ValidationErrors{Errors: fieldErrors}
}

// getErrorMessage returns a user-friendly error message for a validation error
func getErrorMessage(err validator.FieldError) string {
	field := err.Field()

	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, err.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, err.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, err.Param())
	case "eq":
		return fmt.Sprintf("%s must equal %s", field, err.Param())
	case "ne":
		return fmt.Sprintf("%s must not equal %s", field, err.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, err.Param())
	case "ip":
		return fmt.Sprintf("%s must be a valid IP address", field)
	case "ipv4":
		return fmt.Sprintf("%s must be a valid IPv4 address", field)
	case "ipv6":
		return fmt.Sprintf("%s must be a valid IPv6 address", field)
	case "mac":
		return fmt.Sprintf("%s must be a valid MAC address", field)
	case "k8sname":
		return fmt.Sprintf("%s must be a valid Kubernetes name (lowercase alphanumeric, -, or .)", field)
	case "labelkey":
		return fmt.Sprintf("%s must be a valid label key", field)
	case "labelvalue":
		return fmt.Sprintf("%s must be a valid label value", field)
	case "dnssubdomain":
		return fmt.Sprintf("%s must be a valid DNS subdomain", field)
	case "dnslabel":
		return fmt.Sprintf("%s must be a valid DNS label", field)
	default:
		return fmt.Sprintf("%s failed validation (%s)", field, err.Tag())
	}
}

// Kubernetes-style validators

// validateK8sName validates a Kubernetes resource name
// Must be lowercase alphanumeric, hyphen, or dot, 1-253 characters
func validateK8sName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	// Must start and end with alphanumeric
	if !isAlphaNumeric(rune(name[0])) || !isAlphaNumeric(rune(name[len(name)-1])) {
		return false
	}

	// Can contain lowercase alphanumeric, hyphen, or dot
	for _, r := range name {
		if !isK8sNameChar(r) {
			return false
		}
	}

	return true
}

// validateLabelKey validates a Kubernetes label key
// Format: [prefix/]name where name is required and prefix is optional
func validateLabelKey(fl validator.FieldLevel) bool {
	key := fl.Field().String()
	if len(key) == 0 {
		return false
	}

	parts := strings.SplitN(key, "/", 2)

	if len(parts) == 2 {
		// Has prefix
		prefix := parts[0]
		name := parts[1]

		// Prefix must be a valid DNS subdomain
		if !isValidDNSSubdomain(prefix) {
			return false
		}

		// Name part validation
		return isValidLabelName(name)
	}

	// No prefix, just validate name
	return isValidLabelName(key)
}

// validateLabelValue validates a Kubernetes label value
// Must be empty or 1-63 alphanumeric characters, dashes, underscores, or dots
func validateLabelValue(fl validator.FieldLevel) bool {
	value := fl.Field().String()

	// Empty is valid
	if len(value) == 0 {
		return true
	}

	if len(value) > 63 {
		return false
	}

	// Must start and end with alphanumeric if not empty
	if !isAlphaNumeric(rune(value[0])) || !isAlphaNumeric(rune(value[len(value)-1])) {
		return false
	}

	// Can contain alphanumeric, dash, underscore, or dot
	for _, r := range value {
		if !isLabelValueChar(r) {
			return false
		}
	}

	return true
}

// validateDNSSubdomain validates a DNS subdomain (RFC 1123)
// Must be lowercase alphanumeric, hyphen, or dot, max 253 characters
func validateDNSSubdomain(fl validator.FieldLevel) bool {
	subdomain := fl.Field().String()
	return isValidDNSSubdomain(subdomain)
}

// validateDNSLabel validates a DNS label (RFC 1123)
// Must be lowercase alphanumeric or hyphen, 1-63 characters, start and end with alphanumeric
func validateDNSLabel(fl validator.FieldLevel) bool {
	label := fl.Field().String()
	return isValidDNSLabel(label)
}

// Helper functions

func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func isK8sNameChar(r rune) bool {
	return isAlphaNumeric(r) || r == '-' || r == '.'
}

func isLabelValueChar(r rune) bool {
	return isAlphaNumeric(r) || r == '-' || r == '_' || r == '.'
}

func isValidLabelName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	// Must start and end with alphanumeric
	if !isAlphaNumeric(rune(name[0])) || !isAlphaNumeric(rune(name[len(name)-1])) {
		return false
	}

	// Can contain alphanumeric, dash, underscore, or dot
	for _, r := range name {
		if !isLabelValueChar(r) {
			return false
		}
	}

	return true
}

func isValidDNSSubdomain(subdomain string) bool {
	if len(subdomain) == 0 || len(subdomain) > 253 {
		return false
	}

	// Split by dots and validate each label
	labels := strings.Split(subdomain, ".")
	for _, label := range labels {
		if !isValidDNSLabel(label) {
			return false
		}
	}

	return true
}

func isValidDNSLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 {
		return false
	}

	// Must start and end with alphanumeric
	if !isAlphaNumeric(rune(label[0])) || !isAlphaNumeric(rune(label[len(label)-1])) {
		return false
	}

	// Can contain alphanumeric or hyphen
	for _, r := range label {
		if !isAlphaNumeric(r) && r != '-' {
			return false
		}
	}

	return true
}

// RegisterCustomValidator registers a custom validation function
func RegisterCustomValidator(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}

// RegisterCustomValidatorWithMessage registers a custom validation function with a custom message
func RegisterCustomValidatorWithMessage(tag string, fn validator.Func, _ func(validator.FieldError) string) error {
	if err := validate.RegisterValidation(tag, fn); err != nil {
		return err
	}
	// Store message function for later use
	// Note: This would require extending the getErrorMessage function
	return nil
}

// Common validation patterns

var (
	// EmailRegex is a simple email validation regex
	EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	// UUIDRegex validates UUID format
	UUIDRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	// SemanticVersionRegex validates semantic version (e.g., v1.2.3)
	SemanticVersionRegex = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

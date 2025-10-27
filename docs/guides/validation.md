<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Resource Validation in Fabrica

This guide explains how to use Fabrica's validation package to ensure your resources meet required standards.

## Overview

Fabrica provides a powerful validation system that combines:

1. **Declarative Validation**: Using struct tags for common validation rules
2. **Kubernetes-Style Validation**: Built-in validators for K8s naming conventions
3. **Custom Validation**: Interface-based custom validation logic
4. **User-Friendly Errors**: Clear, actionable error messages

## Why Validation Matters

Proper validation:
- Prevents invalid data from entering your system
- Provides immediate feedback to API clients
- Ensures consistency across your resources
- Catches errors early in the request lifecycle
- Improves API usability with clear error messages

## Quick Start

### 1. Define Your Resource with Validation Tags

```go
package main

import (
    "github.com/openchami/fabrica/pkg/resource"
    "github.com/openchami/fabrica/pkg/validation"
)

type Device struct {
    resource.Resource
    Spec DeviceSpec `json:"spec" validate:"required"`
}

type DeviceSpec struct {
    Name       string            `json:"name" validate:"required,k8sname,min=3,max=63"`
    Type       string            `json:"type" validate:"required,oneof=server switch router"`
    IPAddress  string            `json:"ipAddress" validate:"required,ip"`
    MACAddress string            `json:"macAddress,omitempty" validate:"omitempty,mac"`
    Labels     map[string]string `json:"labels" validate:"dive,keys,labelkey,endkeys,labelvalue"`
}
```

### 2. Validate in Your Handlers

```go
func CreateDeviceHandler(w http.ResponseWriter, r *http.Request) {
    var device Device

    // Decode request
    if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Validate the device
    if err := validation.ValidateResource(&device); err != nil {
        // Return structured validation errors
        if validationErrs, ok := err.(validation.ValidationErrors); ok {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "error": "Validation failed",
                "details": validationErrs.Errors,
            })
            return
        }
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Proceed with resource creation
    // ...
}
```

### 3. Handle Validation Errors in Clients

```go
resp, err := http.Post(url, "application/json", body)
if err != nil {
    log.Fatal(err)
}

if resp.StatusCode == http.StatusBadRequest {
    var errorResp struct {
        Error   string                      `json:"error"`
        Details []validation.FieldError `json:"details"`
    }
    json.NewDecoder(resp.Body).Decode(&errorResp)

    for _, fieldErr := range errorResp.Details {
        fmt.Printf("Error in %s: %s\n", fieldErr.Field, fieldErr.Message)
    }
}
```

## Validation Tags Reference

### Required and Optional Fields

```go
type Resource struct {
    Name     string `json:"name" validate:"required"`        // Must be present
    Optional string `json:"optional" validate:"omitempty"`   // Validated only if present
}
```

### String Validation

```go
type Resource struct {
    Name        string `json:"name" validate:"min=3,max=63"`          // Length constraints
    Email       string `json:"email" validate:"email"`                 // Email format
    URL         string `json:"url" validate:"url"`                     // URL format
    Type        string `json:"type" validate:"oneof=a b c"`            // Enumeration
    NoSpaces    string `json:"noSpaces" validate:"excludes= "`         // Exclude characters
    AlphaNum    string `json:"alphaNum" validate:"alphanum"`           // Alphanumeric only
}
```

### Numeric Validation

```go
type Resource struct {
    Port     int     `json:"port" validate:"min=1,max=65535"`      // Range validation
    Age      int     `json:"age" validate:"gte=0,lte=150"`          // Greater/less than or equal
    Score    float64 `json:"score" validate:"min=0.0,max=100.0"`   // Float ranges
    Count    int     `json:"count" validate:"eq=10"`                // Exact value
}
```

### Network Validation

```go
type Resource struct {
    IP      string `json:"ip" validate:"ip"`             // Any IP address
    IPv4    string `json:"ipv4" validate:"ipv4"`         // IPv4 only
    IPv6    string `json:"ipv6" validate:"ipv6"`         // IPv6 only
    CIDR    string `json:"cidr" validate:"cidr"`         // CIDR notation
    MAC     string `json:"mac" validate:"mac"`           // MAC address
    Hostname string `json:"hostname" validate:"hostname"` // Hostname format
}
```

### Kubernetes-Style Validation

```go
type Resource struct {
    // Kubernetes resource name (lowercase, alphanumeric, -, .)
    Name string `json:"name" validate:"k8sname"`

    // DNS label (1-63 chars, alphanumeric or -)
    Label string `json:"label" validate:"dnslabel"`

    // DNS subdomain (max 253 chars, dot-separated labels)
    Domain string `json:"domain" validate:"dnssubdomain"`

    // Label keys and values
    Labels map[string]string `json:"labels" validate:"dive,keys,labelkey,endkeys,labelvalue"`
}
```

### Collection Validation

```go
type Resource struct {
    // Validate each element in slice
    Tags []string `json:"tags" validate:"dive,k8sname"`

    // Validate map keys and values
    Labels map[string]string `json:"labels" validate:"dive,keys,labelkey,endkeys,labelvalue"`

    // Nested struct validation
    Metadata Metadata `json:"metadata" validate:"required"`
}
```

### Cross-Field Validation

```go
type Resource struct {
    Password        string `json:"password" validate:"required,min=8"`
    ConfirmPassword string `json:"confirmPassword" validate:"required,eqfield=Password"`

    StartDate string `json:"startDate" validate:"required"`
    EndDate   string `json:"endDate" validate:"required,gtefield=StartDate"`
}
```

## Custom Validation Logic

For complex validation that can't be expressed with tags, implement the `CustomValidator` interface:

```go
type Device struct {
    resource.Resource
    Spec DeviceSpec `json:"spec" validate:"required"`
}

func (d *Device) Validate(ctx context.Context) error {
    // Custom business rules
    if d.Spec.Type == "server" {
        if d.Spec.MACAddress == "" {
            return errors.New("server devices must have a MAC address")
        }
        if !strings.HasPrefix(d.Spec.Name, "srv-") {
            return errors.New("server names must start with 'srv-'")
        }
    }

    // Context-aware validation
    if deadline, ok := ctx.Deadline(); ok {
        if time.Until(deadline) < time.Second {
            return errors.New("validation timeout approaching")
        }
    }

    return nil
}

// Use ValidateWithContext to run both struct and custom validation
if err := validation.ValidateWithContext(ctx, &device); err != nil {
    // Handle error
}
```

## Validation Error Handling

### Error Structure

```go
type ValidationErrors struct {
    Errors []FieldError `json:"errors"`
}

type FieldError struct {
    Field   string `json:"field"`   // JSON field name
    Tag     string `json:"tag"`     // Validation tag that failed
    Value   string `json:"value"`   // Actual value (optional)
    Message string `json:"message"` // User-friendly message
}
```

### Error Response Format

Return validation errors in a consistent format:

```go
{
    "error": "Validation failed",
    "details": [
        {
            "field": "name",
            "tag": "k8sname",
            "value": "Invalid_Name",
            "message": "name must be a valid Kubernetes name (lowercase alphanumeric, -, or .)"
        },
        {
            "field": "ipAddress",
            "tag": "ip",
            "value": "not-an-ip",
            "message": "ipAddress must be a valid IP address"
        }
    ]
}
```

### Handling in Middleware

Create middleware for consistent error handling:

```go
func ValidationMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Capture panics
        defer func() {
            if err := recover(); err != nil {
                if validationErrs, ok := err.(validation.ValidationErrors); ok {
                    w.Header().Set("Content-Type", "application/json")
                    w.WriteHeader(http.StatusBadRequest)
                    json.NewEncoder(w).Encode(map[string]interface{}{
                        "error":   "Validation failed",
                        "details": validationErrs.Errors,
                    })
                    return
                }
                panic(err) // Re-panic if not a validation error
            }
        }()

        next.ServeHTTP(w, r)
    })
}
```

## Custom Validators

Register your own validators:

```go
import "github.com/go-playground/validator/v10"

func init() {
    // Register a custom validator for semantic versioning
    validation.RegisterCustomValidator("semver", func(fl validator.FieldLevel) bool {
        version := fl.Field().String()
        return validation.SemanticVersionRegex.MatchString(version)
    })
}

type Release struct {
    Version string `json:"version" validate:"required,semver"`
}
```

### Complex Custom Validators

```go
// Validator that checks a value against a database
validation.RegisterCustomValidator("uniqueuser", func(fl validator.FieldLevel) bool {
    username := fl.Field().String()

    // Check database (pseudo-code)
    exists, err := db.UserExists(username)
    if err != nil {
        return false
    }

    return !exists
})

type User struct {
    Username string `json:"username" validate:"required,uniqueuser"`
}
```

## Best Practices

### 1. Validate Early and Often

```go
// Validate as soon as data enters your system
func CreateHandler(w http.ResponseWriter, r *http.Request) {
    var resource MyResource
    json.NewDecoder(r.Body).Decode(&resource)

    // Immediate validation
    if err := validation.ValidateResource(&resource); err != nil {
        // Return error immediately
        return
    }

    // Continue processing
}
```

### 2. Use Specific Validators

```go
// Good: Specific validation
type Device struct {
    Name string `json:"name" validate:"required,k8sname,min=3,max=63"`
    Type string `json:"type" validate:"required,oneof=server switch router"`
}

// Avoid: Too generic
type Device struct {
    Name string `json:"name" validate:"required"`
    Type string `json:"type" validate:"required"`
}
```

### 3. Provide Clear Error Messages

```go
// Good: Return structured errors
if err := validation.ValidateResource(&device); err != nil {
    if validationErrs, ok := err.(validation.ValidationErrors); ok {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error":   "Validation failed",
            "details": validationErrs.Errors,
        })
        return
    }
}

// Avoid: Generic error messages
if err := validation.ValidateResource(&device); err != nil {
    http.Error(w, "Bad request", http.StatusBadRequest)
}
```

### 4. Document Validation Rules

```go
// Good: Clear documentation
type Device struct {
    // Name must be a valid Kubernetes name (lowercase alphanumeric, -, or .)
    // Length: 3-63 characters
    Name string `json:"name" validate:"required,k8sname,min=3,max=63"`

    // Type must be one of: server, switch, router
    Type string `json:"type" validate:"required,oneof=server switch router"`
}
```

### 5. Combine Validation Methods

```go
type Device struct {
    resource.Resource
    Spec DeviceSpec `json:"spec" validate:"required"`
}

func (d *Device) Validate(ctx context.Context) error {
    // Use struct validation for basic rules
    if err := validation.ValidateResource(&d.Spec); err != nil {
        return err
    }

    // Add custom business logic
    if d.Spec.Type == "server" && d.Spec.MACAddress == "" {
        return errors.New("servers must have a MAC address")
    }

    return nil
}
```

## Integration Examples

### With HTTP Handlers

```go
func (s *Server) CreateDevice(w http.ResponseWriter, r *http.Request) {
    var device Device

    if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    if err := validation.ValidateWithContext(r.Context(), &device); err != nil {
        s.handleValidationError(w, err)
        return
    }

    // Store device
    if err := s.storage.Create(r.Context(), &device); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(device)
}

func (s *Server) handleValidationError(w http.ResponseWriter, err error) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusBadRequest)

    if validationErrs, ok := err.(validation.ValidationErrors); ok {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error":   "Validation failed",
            "details": validationErrs.Errors,
        })
    } else {
        json.NewEncoder(w).Encode(map[string]string{
            "error": err.Error(),
        })
    }
}
```

### With Code Generation

Update your templates to include validation:

```go
// In handlers template
func (s *Server) Create{{.ResourceName}}(w http.ResponseWriter, r *http.Request) {
    var resource {{.ResourceType}}

    if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Auto-generated validation call
    if err := validation.ValidateWithContext(r.Context(), &resource); err != nil {
        handleValidationError(w, err)
        return
    }

    // Continue with resource creation...
}
```

## Testing Validation

### Unit Tests

```go
func TestDeviceValidation(t *testing.T) {
    tests := []struct {
        name    string
        device  Device
        wantErr bool
        errField string
    }{
        {
            name: "valid device",
            device: Device{
                Spec: DeviceSpec{
                    Name:      "my-server",
                    Type:      "server",
                    IPAddress: "192.168.1.1",
                },
            },
            wantErr: false,
        },
        {
            name: "invalid name",
            device: Device{
                Spec: DeviceSpec{
                    Name:      "Invalid_Name",
                    Type:      "server",
                    IPAddress: "192.168.1.1",
                },
            },
            wantErr:  true,
            errField: "name",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validation.ValidateResource(&tt.device)
            if (err != nil) != tt.wantErr {
                t.Errorf("wanted error: %v, got: %v", tt.wantErr, err)
            }

            if tt.wantErr {
                validationErrs := err.(validation.ValidationErrors)
                if validationErrs.Errors[0].Field != tt.errField {
                    t.Errorf("wanted error in field %s, got %s",
                        tt.errField, validationErrs.Errors[0].Field)
                }
            }
        })
    }
}
```

## Performance Tips

1. **Validation is Fast**: The validator compiles rules once and caches them
2. **Reuse Validators**: Don't create new validators for each validation
3. **Early Returns**: Return on first error for faster failure
4. **Context Timeouts**: Use context with timeouts for long-running custom validation

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := validation.ValidateWithContext(ctx, &resource); err != nil {
    // Handle error
}
```

## Additional Resources

- [Package Documentation](../pkg/validation/README.md)
- [go-playground/validator Documentation](https://github.com/go-playground/validator)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [RFC 1123: DNS Requirements](https://tools.ietf.org/html/rfc1123)

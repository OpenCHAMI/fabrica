<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Validation Package

The validation package provides comprehensive validation for Fabrica resources, combining struct-tag based validation with custom Kubernetes-style validators.

## Features

- **Struct Tag Validation**: Uses [go-playground/validator](https://github.com/go-playground/validator) for declarative validation
- **Kubernetes-style Validators**: Custom validators for K8s resource names, labels, and DNS formats
- **Context-Aware Validation**: Support for custom validation logic via the `CustomValidator` interface
- **User-Friendly Error Messages**: Formatted error messages that are easy to understand
- **Extensible**: Register your own custom validators

## Quick Start

### Basic Struct Validation

```go
import "github.com/alexlovelltroy/fabrica/pkg/validation"

type Device struct {
    Name        string `json:"name" validate:"required,k8sname"`
    Description string `json:"description" validate:"max=100"`
    Email       string `json:"email" validate:"omitempty,email"`
}

device := Device{
    Name:  "my-device",
    Email: "admin@example.com",
}

if err := validation.ValidateResource(&device); err != nil {
    // Handle validation error
    fmt.Println(err)
}
```

### Custom Validation Logic

Implement the `CustomValidator` interface for complex validation:

```go
type Device struct {
    Name string `json:"name" validate:"required,k8sname"`
    Type string `json:"type" validate:"required"`
}

func (d *Device) Validate(ctx context.Context) error {
    // Custom business logic validation
    if d.Type == "router" && !strings.HasPrefix(d.Name, "rtr-") {
        return errors.New("router devices must have names starting with 'rtr-'")
    }
    return nil
}

// Use ValidateWithContext to run both struct and custom validation
if err := validation.ValidateWithContext(ctx, &device); err != nil {
    // Handle error
}
```

## Built-in Validators

### Standard Validators

All standard go-playground/validator tags are supported:

- `required`: Field must be present and not empty
- `omitempty`: Skip validation if field is empty
- `min=N`: Minimum value/length
- `max=N`: Maximum value/length
- `len=N`: Exact length
- `eq=value`: Must equal value
- `ne=value`: Must not equal value
- `oneof=a b c`: Must be one of the listed values
- `email`: Valid email address
- `url`: Valid URL
- `ip`, `ipv4`, `ipv6`: IP address validation
- `mac`: MAC address validation

### Kubernetes-style Validators

#### `k8sname`

Validates Kubernetes resource names (DNS-1123 subdomain format):
- Lowercase alphanumeric characters, `-`, or `.`
- Must start and end with alphanumeric
- 1-253 characters

```go
type Resource struct {
    Name string `json:"name" validate:"required,k8sname"`
}
```

Valid examples: `my-resource`, `example.com`, `resource-123`
Invalid examples: `MyResource`, `-invalid`, `under_score`

#### `labelkey`

Validates Kubernetes label keys with optional prefix:
- Format: `[prefix/]name`
- Prefix must be a valid DNS subdomain (if present)
- Name must be 1-63 alphanumeric characters, `-`, `_`, or `.`
- Must start and end with alphanumeric

```go
type Resource struct {
    Labels map[string]string `json:"labels" validate:"dive,keys,labelkey,endkeys"`
}
```

Valid examples: `app`, `app-name`, `example.com/app`, `version`
Invalid examples: ``, `-app`, `app/name/extra`

#### `labelvalue`

Validates Kubernetes label values:
- Empty or 1-63 alphanumeric characters, `-`, `_`, or `.`
- Must start and end with alphanumeric if not empty

```go
type Resource struct {
    Labels map[string]string `json:"labels" validate:"dive,labelvalue"`
}
```

Valid examples: ``, `v1.0`, `production`, `app-123`
Invalid examples: `-invalid`, `too-long-value-that-exceeds-sixty-three-characters-limit-here`

#### `dnssubdomain`

Validates DNS subdomain (RFC 1123):
- Lowercase alphanumeric or `-`
- Maximum 253 characters
- Each label separated by `.` must be valid

```go
type Resource struct {
    Domain string `json:"domain" validate:"dnssubdomain"`
}
```

Valid examples: `example.com`, `api.example.com`, `my-service`
Invalid examples: `Example.com`, `-invalid`, `too.many.dots..`

#### `dnslabel`

Validates DNS label (RFC 1123):
- Lowercase alphanumeric or `-`
- 1-63 characters
- Must start and end with alphanumeric

```go
type Resource struct {
    Label string `json:"label" validate:"dnslabel"`
}
```

Valid examples: `example`, `my-label`, `label123`
Invalid examples: `Example`, `-invalid`, `label.with.dots`

## Validating Maps and Slices

### Map Validation

Validate both keys and values in maps:

```go
type Resource struct {
    Labels map[string]string `json:"labels" validate:"dive,keys,labelkey,endkeys,labelvalue"`
}
```

This validates:
1. Each key is a valid label key
2. Each value is a valid label value

### Slice Validation

Validate slice elements:

```go
type Resource struct {
    Names []string `json:"names" validate:"dive,k8sname"`
}
```

Each element in the slice must be a valid Kubernetes name.

## Error Handling

The validation package returns structured errors:

```go
err := validation.ValidateResource(&resource)
if err != nil {
    if validationErrs, ok := err.(validation.ValidationErrors); ok {
        for _, fieldErr := range validationErrs.Errors {
            fmt.Printf("Field: %s, Error: %s\n", fieldErr.Field, fieldErr.Message)
        }
    }
}
```

Error structure:

```go
type ValidationErrors struct {
    Errors []FieldError
}

type FieldError struct {
    Field   string // JSON field name
    Tag     string // Validation tag that failed
    Value   string // Actual value (if applicable)
    Message string // User-friendly error message
}
```

## Custom Validators

### Registering a Custom Validator

```go
import "github.com/go-playground/validator/v10"

// Register a custom validator
err := validation.RegisterCustomValidator("customtag", func(fl validator.FieldLevel) bool {
    value := fl.Field().String()
    // Your validation logic
    return isValid(value)
})

// Use in struct tags
type Resource struct {
    Field string `json:"field" validate:"customtag"`
}
```

### Common Validation Patterns

The package exports common regex patterns for custom validation:

```go
// Email validation
validation.EmailRegex.MatchString(email)

// UUID validation
validation.UUIDRegex.MatchString(uuid)

// Semantic version validation (e.g., v1.2.3)
validation.SemanticVersionRegex.MatchString(version)
```

## Integration with Fabrica Handlers

In your resource handlers, add validation:

```go
func CreateDeviceHandler(w http.ResponseWriter, r *http.Request) {
    var device Device
    if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Validate the resource
    if err := validation.ValidateWithContext(r.Context(), &device); err != nil {
        if validationErrs, ok := err.(validation.ValidationErrors); ok {
            // Return structured error response
            w.Header().Set("Content-Type", "application/json")
            w.WriteStatus(http.StatusBadRequest)
            json.NewEncoder(w).Encode(validationErrs)
            return
        }
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Proceed with creating the resource
    // ...
}
```

## Best Practices

1. **Validate Early**: Validate resources as soon as they're received
2. **Use Context**: Use `ValidateWithContext` for validation that may need cancellation or timeouts
3. **Structured Errors**: Return the `ValidationErrors` type in API responses for clear client feedback
4. **Combine Validators**: Use multiple validators on a single field (e.g., `validate:"required,k8sname,min=3"`)
5. **Custom Logic**: Implement `CustomValidator` for complex business rules that can't be expressed with tags
6. **Document Requirements**: Include validation requirements in your API documentation

## Examples

### Complete Resource Validation

```go
type Device struct {
    Metadata Metadata   `json:"metadata" validate:"required"`
    Spec     DeviceSpec `json:"spec" validate:"required"`
}

type Metadata struct {
    Name        string            `json:"name" validate:"required,k8sname"`
    Labels      map[string]string `json:"labels" validate:"dive,keys,labelkey,endkeys,labelvalue"`
    Annotations map[string]string `json:"annotations,omitempty"`
}

type DeviceSpec struct {
    Type        string   `json:"type" validate:"required,oneof=server router switch"`
    IPAddress   string   `json:"ipAddress" validate:"required,ip"`
    MACAddress  string   `json:"macAddress,omitempty" validate:"omitempty,mac"`
    Tags        []string `json:"tags,omitempty" validate:"dive,labelvalue"`
}

func (d *Device) Validate(ctx context.Context) error {
    // Custom validation: servers must have MAC addresses
    if d.Spec.Type == "server" && d.Spec.MACAddress == "" {
        return errors.New("server devices must have a MAC address")
    }
    return nil
}

// Usage
device := Device{...}
if err := validation.ValidateWithContext(ctx, &device); err != nil {
    // Handle validation errors
}
```

## Testing

The validation package includes comprehensive tests. Run them with:

```bash
go test ./pkg/validation/... -v
```

## Performance Considerations

- Validators are compiled once at initialization and reused
- Struct tag parsing is done once per struct type
- Custom validators are cached
- For high-throughput APIs, consider caching validation results for identical payloads

## References

- [go-playground/validator Documentation](https://github.com/go-playground/validator)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [RFC 1123: DNS Requirements](https://tools.ietf.org/html/rfc1123)

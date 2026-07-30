<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Middleware Customization Guide

> Learn how to extend and customize generated middleware in your Fabrica projects.

## Table of Contents

- [Overview](#overview)
- [Generated Middleware](#generated-middleware)
- [Request Body Limits](#request-body-limits)
- [Middleware Order](#middleware-order)
- [Adding Custom Middleware](#adding-custom-middleware)
- [Per-Resource Middleware](#per-resource-middleware)
- [Common Patterns](#common-patterns)
- [Best Practices](#best-practices)

## Overview

Fabrica generates middleware for common API concerns (validation, conditional requests, versioning, events). You can add your own custom middleware to the chain to implement authentication, logging, rate limiting, and other cross-cutting concerns.

**Key Points:**
- Generated middleware lives in `internal/middleware/*_generated.go`
- Never edit generated files - they're overwritten on each `fabrica generate`
- Add custom middleware by editing route registration in `cmd/server/routes_generated.go` or wrapping in `main.go`
- Middleware order matters for correct behavior

## Generated Middleware

### What Gets Generated

When you run `fabrica generate`, the following middleware is automatically created based on your `.fabrica.yaml` configuration:

#### 1. Validation Middleware (`validation_middleware_generated.go`)

Validates incoming requests against struct tags and custom validation logic.

```go
// Generated validation middleware
func ValidationMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Validate request body
        // Return 400 if validation fails (strict mode)
        // Log warning if validation fails (warn mode)
        next.ServeHTTP(w, r)
    })
}
```

**Configuration:**
```yaml
# .fabrica.yaml
features:
  validation:
    enabled: true
    mode: strict  # strict | warn | disabled
```

#### 2. Conditional Middleware (`conditional_middleware_generated.go`)

Handles ETags and conditional request headers (If-Match, If-None-Match, etc.).

```go
// Generated conditional middleware
func ConditionalMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check If-Match, If-None-Match headers
        // Generate and set ETag headers
        // Return 304 Not Modified or 412 Precondition Failed if needed
        next.ServeHTTP(w, r)
    })
}
```

**Configuration:**
```yaml
features:
  conditional:
    enabled: true
    etag_algorithm: sha256  # sha256 | md5
```

#### 3. Versioning Middleware (`versioning_middleware_generated.go`)

Negotiates API versions and performs conversions between hub and spoke versions.

```go
// Generated versioning middleware
func VersioningMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Parse apiVersion from body (POST/PUT/PATCH), URL version, or Accept header
        // Set requested version in context
        // Handlers will convert from hub version if needed
        next.ServeHTTP(w, r)
    })
}
```

**Configuration:**
```yaml
features:
  versioning:
    enabled: true
    strategy: header  # header | path | query
```

#### 4. Events Middleware (`event_middleware_generated.go`)

Publishes CloudEvents for resource lifecycle operations.

```go
// Generated in handlers, not as separate middleware
// Automatically publishes events after successful operations:
// - io.fabrica.{resource}.created
// - io.fabrica.{resource}.updated
// - io.fabrica.{resource}.deleted
```

**Configuration:**
```yaml
features:
  events:
    enabled: true
    bus_type: memory  # memory | nats | kafka
```

## Request Body Limits

Generated servers install request-body limiting as the outermost router middleware, before version negotiation and all PATCH handling. Positive `Content-Length` values above the selected limit are rejected immediately. Streaming or chunked bodies are wrapped with `http.MaxBytesReader`, so middleware and handlers cannot buffer beyond the selected limit.

The compatibility default is 16 MiB. This accommodates multi-megabyte inventory payloads such as `DiscoverySnapshot` raw discovery data while retaining a finite denial-of-service boundary. Configure the global limit with the generated `--request-body-max-bytes` flag, the service-prefixed `*_REQUEST_BODY_MAX_BYTES` environment variable, or the runtime YAML key:

```yaml
request_body_max_bytes: 16777216
request_body_max_bytes_by_resource:
  DiscoverySnapshot: 33554432
```

Per-resource keys are exact generated resource Kind names. Unknown resource names and nonpositive global or per-resource values fail server startup before storage initialization. Oversized bodies return HTTP 413 with the stable response `{"error":"request body too large","code":413}` and do not reach storage writes.

Generated JSON handlers accept exactly one JSON value. They consume the remainder of the bounded stream to verify EOF, which rejects a second value, malformed trailing bytes, and trailing whitespace that pushes the total body above the configured limit.

Executable coverage is anchored by `TestGeneratedHandlers_bound_every_request_body_with_stable_413_contract` for generated structure and `TestDedicatedSecurity_generated_adapter_runtime` for declared/chunked limits, per-resource overrides, inventory-scale payloads, trailing-body rejection, stable 413 responses, and no-write behavior.

## Middleware Order

**Middleware executes in the order it's registered.** The correct order ensures each middleware has the context it needs:

### Recommended Order

```
1. Request body limit (outermost - bounds every body reader)
2. Logging            (log accepted requests)
3. Recovery/Panic     (catch panics early)
4. CORS               (handle preflight requests)
5. Authentication     (verify identity)
6. Authorization      (check permissions)
7. Rate Limiting      (throttle requests)
8. Validation         (validate input)
9. Versioning         (negotiate API version)
10. Conditional       (check ETags, timestamps)
11. Handler           (business logic)
```

### Why Order Matters

**Wrong Order Example:**
```go
// ❌ Bad: Validation before authentication
r.Use(ValidationMiddleware)
r.Use(AuthMiddleware)  // Validates unauthenticated requests!
```

**Correct Order:**
```go
// ✅ Good: Authentication before validation
r.Use(AuthMiddleware)  // Check identity first
r.Use(ValidationMiddleware)  // Then validate for authorized user
```

## How Middleware Inheritance Works

**Important:** Fabrica uses Chi router groups to separate public and protected routes. Understanding how middleware flows through these groups is critical for security.

### Correct Pattern (Used by Fabrica)

The generated `main.go` creates separate router groups for public and protected endpoints:

```go
// cmd/server/main.go (generated)
func main() {
    r := chi.NewRouter()

    // Global middleware (applies to all routes)
    r.Use(bodyLimitMiddleware) // Must remain outermost
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Public endpoints (no auth required)
    r.Group(func(public chi.Router) {
        public.Get("/health", healthHandler)
        public.Get("/openapi.json", ServeOpenAPISpec)
    })

    // Protected endpoints (auth required)
    r.Group(func(protected chi.Router) {
        if config.AuthEnabled {
            protected.Use(authnMiddleware)  // Apply auth middleware
            protected.Use(authzMiddleware)
        }

        // Register generated routes - they inherit middleware
        RegisterGeneratedRoutes(protected)  // ← Middleware flows here
    })
}
```

### Route Registration (Fixed in v0.4.6+)

The `RegisterGeneratedRoutes` function receives a router with middleware already applied:

```go
// cmd/server/routes_generated.go
func RegisterGeneratedRoutes(r chi.Router) {
    // Routes inherit middleware from the router passed by main.go
    r.Route("/devices", func(resource chi.Router) {
        resource.Get("/", GetDevices)
        resource.Post("/", CreateDevice)
        // ... more routes
    })
}
```

**Key Point:** Routes are registered directly on the parameter router `r`, which already has auth middleware applied in `main.go`. This ensures all protected routes enforce authentication.

### Common Mistake (Fixed in v0.4.6)

Prior to v0.4.6, the template incorrectly created a nested group that shadowed the router parameter:

```go
// ❌ WRONG (old template - do not use)
func RegisterGeneratedRoutes(r chi.Router) {
    r.Group(func(protected chi.Router) {  // ← Creates NEW router
        // Routes registered here LOSE middleware from 'r' parameter
        protected.Route("/devices", ...)  // ← No auth enforced!
    })
}
```

**Problem:** The nested `r.Group()` creates a new router instance, and the inner `protected` variable shadows the parameter, causing middleware to be lost.

**If you see this pattern in your generated code, regenerate with `fabrica generate` using v0.4.6 or later.**

## Adding Custom Middleware

### Method 1: Global Middleware (Recommended)

Add middleware to all routes by editing `cmd/server/main.go`:

```go
// cmd/server/main.go
func main() {
    router := chi.NewRouter()

    // Add your custom middleware BEFORE generated routes
    router.Use(LoggingMiddleware)      // Your custom logger
    router.Use(RecoveryMiddleware)     // Your panic recovery
    router.Use(CORSMiddleware)         // Your CORS config
    router.Use(AuthenticationMiddleware)  // Your auth

    // Register generated routes (includes generated middleware)
    RegisterRoutes(router, storage, eventBus)

    // Start server...
}
```

### Method 2: Route-Specific Middleware

Wrap specific routes in `cmd/server/routes_generated.go` (but be aware this file is regenerated):

**Better approach:** Edit `main.go` to add middleware to specific paths:

```go
// cmd/server/main.go
func main() {
    router := chi.NewRouter()

    // Global middleware
    router.Use(LoggingMiddleware)

    // Register base routes
    RegisterRoutes(router, storage, eventBus)

    // Add middleware to specific routes
    router.Group(func(r chi.Router) {
        r.Use(AdminOnlyMiddleware)  // Extra auth for admin routes
        r.Post("/devices/import", ImportDevicesHandler)
        r.Post("/devices/export", ExportDevicesHandler)
    })

    http.ListenAndServe(":8080", router)
}
```

### Method 3: Per-Resource Middleware

Use Chi's sub-routers to apply middleware to specific resource types:

```go
// cmd/server/main.go
func RegisterCustomRoutes(router chi.Router, storage storage.StorageBackend) {
    // Devices need extra auth
    router.Route("/devices", func(r chi.Router) {
        r.Use(DeviceAuthMiddleware)
        r.Post("/", CreateDeviceHandler)
        r.Get("/", ListDevicesHandler)
    })

    // Users have rate limiting
    router.Route("/users", func(r chi.Router) {
        r.Use(RateLimitMiddleware(100, time.Minute))
        r.Post("/", CreateUserHandler)
        r.Get("/", ListUsersHandler)
    })
}
```

## Common Patterns

### 1. Logging Middleware

Log all requests with timing information:

```go
// internal/middleware/logging.go
package middleware

import (
    "log"
    "net/http"
    "time"
)

func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Wrap ResponseWriter to capture status code
        ww := &responseWriter{ResponseWriter: w}

        // Call next handler
        next.ServeHTTP(ww, r)

        // Log after request completes
        log.Printf("%s %s %s %d %s",
            r.Method,
            r.URL.Path,
            r.RemoteAddr,
            ww.statusCode,
            time.Since(start),
        )
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}
```

### 2. Authentication Middleware

Verify JWT tokens:

```go
// internal/middleware/auth.go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserContextKey = contextKey("user")

func AuthenticationMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract token from Authorization header
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Missing authorization header", http.StatusUnauthorized)
                return
            }

            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            if tokenString == authHeader {
                http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
                return
            }

            // Parse and validate token
            token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
                return jwtSecret, nil
            })

            if err != nil || !token.Valid {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            // Extract claims and add to context
            if claims, ok := token.Claims.(jwt.MapClaims); ok {
                ctx := context.WithValue(r.Context(), UserContextKey, claims)
                next.ServeHTTP(w, r.WithContext(ctx))
            } else {
                http.Error(w, "Invalid token claims", http.StatusUnauthorized)
            }
        })
    }
}

// Helper to get user from context
func GetUser(r *http.Request) (jwt.MapClaims, bool) {
    user, ok := r.Context().Value(UserContextKey).(jwt.MapClaims)
    return user, ok
}
```

### 3. Rate Limiting Middleware

Throttle requests per IP:

```go
// internal/middleware/ratelimit.go
package middleware

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     rate.Limit(rps),
        burst:    burst,
    }
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
    rl.mu.RLock()
    limiter, exists := rl.limiters[key]
    rl.mu.RUnlock()

    if !exists {
        rl.mu.Lock()
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[key] = limiter
        rl.mu.Unlock()
    }

    return limiter
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        limiter := rl.getLimiter(r.RemoteAddr)

        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**Usage:**
```go
rateLimiter := NewRateLimiter(100, 200) // 100 req/sec, burst of 200
router.Use(rateLimiter.Middleware)
```

### 4. CORS Middleware

Handle cross-origin requests:

```go
// internal/middleware/cors.go
package middleware

import "net/http"

func CORSMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, If-Match, If-None-Match")
        w.Header().Set("Access-Control-Expose-Headers", "ETag, Last-Modified")

        // Handle preflight
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### 5. Request ID Middleware

Add unique ID to each request for tracing:

```go
// internal/middleware/requestid.go
package middleware

import (
    "context"
    "net/http"

    "github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"
const RequestIDContextKey = contextKey("requestID")

func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check for existing request ID
        requestID := r.Header.Get(RequestIDHeader)
        if requestID == "" {
            requestID = uuid.New().String()
        }

        // Set in response header
        w.Header().Set(RequestIDHeader, requestID)

        // Add to context
        ctx := context.WithValue(r.Context(), RequestIDContextKey, requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Helper to get request ID
func GetRequestID(r *http.Request) string {
    if requestID, ok := r.Context().Value(RequestIDContextKey).(string); ok {
        return requestID
    }
    return ""
}
```

## Per-Resource Middleware

Apply middleware only to specific resource types:

```go
// cmd/server/main.go
func main() {
    router := chi.NewRouter()

    // Global middleware
    router.Use(LoggingMiddleware)
    router.Use(AuthenticationMiddleware(jwtSecret))

    // Devices - require admin role
    router.Route("/devices", func(r chi.Router) {
        r.Use(RequireRole("admin"))
        RegisterDeviceRoutes(r, storage)
    })

    // Users - open to authenticated users
    router.Route("/users", func(r chi.Router) {
        RegisterUserRoutes(r, storage)
    })

    // Products - rate limited
    router.Route("/products", func(r chi.Router) {
        r.Use(rateLimiter.Middleware)
        RegisterProductRoutes(r, storage)
    })

    http.ListenAndServe(":8080", router)
}

func RequireRole(role string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, ok := GetUser(r)
            if !ok {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            roles, _ := user["roles"].([]interface{})
            hasRole := false
            for _, r := range roles {
                if r.(string) == role {
                    hasRole = true
                    break
                }
            }

            if !hasRole {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

## Best Practices

### 1. Don't Edit Generated Files

```
✅ Good:
  cmd/server/main.go            (add middleware here)
  internal/middleware/custom.go (your middleware)

❌ Bad:
  cmd/server/routes_generated.go        (regenerated)
  internal/middleware/*_generated.go    (regenerated)
```

### 2. Use Context for Request-Scoped Data

```go
// Store user info in context
ctx := context.WithValue(r.Context(), UserKey, user)
r = r.WithContext(ctx)

// Retrieve in handlers
user := r.Context().Value(UserKey).(*User)
```

### 3. Handle Errors Consistently

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractToken(r)
        if token == "" {
            // Use consistent error format
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Missing authentication token",
            })
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### 4. Make Middleware Configurable

```go
// Allow configuration
func RateLimitMiddleware(requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
    limiter := rate.NewLimiter(rate.Limit(requestsPerSecond), burst)

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// Use with different configs
router.Use(RateLimitMiddleware(100, 200))  // More permissive
adminRouter.Use(RateLimitMiddleware(10, 20))  // More restrictive
```

### 5. Test Middleware Independently

```go
func TestAuthMiddleware(t *testing.T) {
    // Create test handler
    testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    // Wrap with middleware
    handler := AuthMiddleware(jwtSecret)(testHandler)

    // Test without token
    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    if w.Code != http.StatusUnauthorized {
        t.Errorf("Expected 401, got %d", w.Code)
    }

    // Test with valid token
    req.Header.Set("Authorization", "Bearer " + validToken)
    w = httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", w.Code)
    }
}
```

## See Also

- [Getting Started Guide](getting-started.md) - Basic setup
- [Validation Guide](validation.md) - Generated validation middleware
- [Conditional Requests](conditional-and-patch.md) - Generated conditional middleware
- [Versioning Guide](versioning.md) - Generated versioning middleware
- [Events Guide](events.md) - Event publishing in handlers

---

**Next Steps:**
- Implement authentication middleware for your project
- Add logging and monitoring
- Configure rate limiting based on your needs
- Test middleware independently before integration

// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package authz defines the minimal runtime contract that Fabrica-generated
// services use to call TokenSmith-managed authorization.
//
// Fabrica intentionally does NOT implement a policy engine. Generated services
// should delegate policy evaluation to TokenSmith (which may internally use
// Casbin or other mechanisms).
package authz

import (
	"context"
	"net/http"
)

// Mode controls how authorization decisions affect request handling.
//
//   - enforce: deny requests when policy says deny, or when the request cannot
//     be classified (ok=false) or an internal error occurs.
//   - shadow: never deny; emit a structured decision log/event instead.
//
// Generated code will treat the mode as case-insensitive.
type Mode string

// ModeEnforce means authorization decisions are enforced, and requests may be denied.
// ModeShadow means authorization decisions are logged but not enforced; all requests are allowed.
const (
	ModeEnforce Mode = "enforce"
	ModeShadow  Mode = "shadow"
)

// Tuple is the unit of authorization used by Fabrica-generated services.
//
// Decision tuple format (subject, object, action):
//   - subject: caller identity, typically derived from JWT claims (e.g. sub)
//   - object: stable resource identifier (prefer chi RoutePattern; fallback path)
//   - action: typically the HTTP method (GET/POST/PUT/PATCH/DELETE)
//
// TokenSmith is expected to consume these values as its enforcement inputs.
type Tuple struct {
	Subject string
	Object  string
	Action  string
}

// Decision represents the outcome of a policy evaluation.
type Decision struct {
	Allowed bool

	// Reason is a short human-readable summary suitable for logs.
	Reason string

	// Metadata may contain structured fields for logging/auditing.
	Metadata map[string]any
}

// Adapter is the minimal interface generated services depend on for AuthZ.
//
// Implementations are expected to be thin wrappers around TokenSmith APIs.
//
// Error handling contract:
//   - If err != nil, this indicates an internal failure to evaluate policy.
//     Generated code treats this as deny in enforce mode; in shadow mode it is
//     logged and the request is allowed.
//   - If err == nil and Decision.Allowed == false, this indicates a clean deny.
//     Generated code returns 403 in enforce mode; in shadow mode it is logged
//     and the request is allowed.
type Adapter interface {
	Authorize(ctx context.Context, r *http.Request, t Tuple) (Decision, error)
}

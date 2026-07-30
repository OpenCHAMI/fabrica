// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

// Package httpbody provides bounded HTTP request-body reads shared by generated
// routers and middleware that inspect or decode request payloads.
package httpbody

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// DefaultMaxBytes is 16 MiB so inventory-bearing resources such as
// DiscoverySnapshot fit by default while accidental or hostile bodies remain bounded.
const DefaultMaxBytes int64 = 16 << 20

var (
	// ErrInvalidLimit identifies a nonpositive request-body limit.
	ErrInvalidLimit = errors.New("request body limit must be positive")
	// ErrMultipleJSONValues identifies a request containing more than one JSON value.
	ErrMultipleJSONValues = errors.New("request body must contain exactly one JSON value")
)

type limitContextKey struct{}

type limitState struct {
	maxBytes int64
}

// Apply validates maxBytes, rejects an oversized declared Content-Length, and
// wraps the body exactly once with http.MaxBytesReader.
func Apply(w http.ResponseWriter, r *http.Request, maxBytes int64) error {
	if maxBytes <= 0 {
		return fmt.Errorf("%w: %d", ErrInvalidLimit, maxBytes)
	}
	if r.ContentLength > maxBytes {
		return &http.MaxBytesError{Limit: maxBytes}
	}
	if _, ok := r.Context().Value(limitContextKey{}).(limitState); ok {
		return nil
	}
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	}
	*r = *r.WithContext(context.WithValue(r.Context(), limitContextKey{}, limitState{maxBytes: maxBytes}))
	return nil
}

// MaxBytes returns the configured request limit or the compatibility default.
func MaxBytes(r *http.Request) int64 {
	if state, ok := r.Context().Value(limitContextKey{}).(limitState); ok {
		return state.maxBytes
	}
	return DefaultMaxBytes
}

// ReadAll reads and restores at most the configured number of bytes.
func ReadAll(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := Apply(w, r, MaxBytes(r)); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, err
}

// DecodeOne decodes exactly one JSON value and consumes the remaining stream to
// verify EOF, enforcing the limit across trailing whitespace and trailing data.
func DecodeOne(w http.ResponseWriter, r *http.Request, destination any) error {
	if err := Apply(w, r, MaxBytes(r)); err != nil {
		return err
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing json.RawMessage
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return ErrMultipleJSONValues
	}
	return err
}

// IsTooLarge reports whether err contains http.MaxBytesError.
func IsTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

// WriteTooLarge writes the stable generated-server 413 response contract.
func WriteTooLarge(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}{Error: "request body too large", Code: http.StatusRequestEntityTooLarge})
}

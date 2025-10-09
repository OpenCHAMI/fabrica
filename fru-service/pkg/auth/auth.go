// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package auth

import (
	"log"
	"net/http"
)

// Config holds authentication configuration
type Config struct {
	Enabled        bool   `mapstructure:"enabled"`
	NonEnforcing   bool   `mapstructure:"non_enforcing"`
	JWTPublicKey   string `mapstructure:"jwt_public_key"`
	JWKSUrl        string `mapstructure:"jwks_url"`
	JWTIssuer      string `mapstructure:"jwt_issuer"`
	JWTAudience    string `mapstructure:"jwt_audience"`
}

// DefaultConfig returns the default authentication configuration
func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		NonEnforcing: false,
		JWTIssuer:    "",
		JWTAudience:  "",
		JWKSUrl:      "",
		JWTPublicKey: "",
	}
}

// CreateMiddleware creates an authentication middleware function
func (c Config) CreateMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !c.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// TODO: Implement TokenSmith middleware integration
			// This is a placeholder implementation
			if c.NonEnforcing {
				logger.Printf("Auth middleware (non-enforcing): would validate request %s %s", r.Method, r.URL.Path)
				next.ServeHTTP(w, r)
				return
			}

			// For now, pass through all requests
			// In a real implementation, this would:
			// 1. Extract JWT from Authorization header
			// 2. Validate JWT signature using JWKS or static key
			// 3. Verify issuer and audience claims
			// 4. Extract user scopes/roles
			// 5. Set user context for downstream handlers

			logger.Printf("Auth middleware: validating request %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
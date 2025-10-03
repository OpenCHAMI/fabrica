// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

// Product represents an item in your store catalog.
// This is a SIMPLE example - just a plain Go struct with no Kubernetes concepts.
//
// In simple mode, you don't need:
// - resource.Resource embedding
// - Spec/Status separation
// - Labels or annotations
// - Metadata fields
//
// Just define your data and let Fabrica generate the REST API!
type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	InStock     bool    `json:"inStock"`
}

// Note: In real usage, you'd run:
//   fabrica add resource Product
//
// This would analyze this struct and generate:
//   - HTTP handlers (Create, Get, Update, Delete, List)
//   - Storage layer (in-memory by default)
//   - Routes configuration
//   - Main server file
//
// Total generated code: ~200 lines
// Code you write: Just this struct definition above!

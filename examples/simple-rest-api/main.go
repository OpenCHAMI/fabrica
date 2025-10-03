// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// This file simulates what Fabrica would generate with:
//   fabrica add resource Product
//
// In real usage, you would NEVER write this file - it's auto-generated!
// We include it here for educational purposes only.

// ProductStorage provides thread-safe in-memory storage for products
type ProductStorage struct {
	mu       sync.RWMutex
	products map[string]Product
}

// NewProductStorage creates a new product storage instance
func NewProductStorage() *ProductStorage {
	return &ProductStorage{
		products: make(map[string]Product),
	}
}

// Create adds a new product
func (s *ProductStorage) Create(p Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.products[p.ID]; exists {
		return fmt.Errorf("product with ID %s already exists", p.ID)
	}

	s.products[p.ID] = p
	return nil
}

// Get retrieves a product by ID
func (s *ProductStorage) Get(id string) (Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.products[id]
	if !exists {
		return Product{}, fmt.Errorf("product with ID %s not found", id)
	}

	return p, nil
}

// Update modifies an existing product
func (s *ProductStorage) Update(p Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.products[p.ID]; !exists {
		return fmt.Errorf("product with ID %s not found", p.ID)
	}

	s.products[p.ID] = p
	return nil
}

// Delete removes a product
func (s *ProductStorage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.products[id]; !exists {
		return fmt.Errorf("product with ID %s not found", id)
	}

	delete(s.products, id)
	return nil
}

// List returns all products
func (s *ProductStorage) List() []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()

	products := make([]Product, 0, len(s.products))
	for _, p := range s.products {
		products = append(products, p)
	}

	return products
}

// ProductHandlers provides HTTP handlers for product operations
type ProductHandlers struct {
	storage *ProductStorage
}

// NewProductHandlers creates a new handler instance
func NewProductHandlers(storage *ProductStorage) *ProductHandlers {
	return &ProductHandlers{storage: storage}
}

// Create handles POST /products
func (h *ProductHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.storage.Create(product); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(product); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// Get handles GET /products/{id}
func (h *ProductHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	product, err := h.storage.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(product); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// Update handles PUT /products/{id}
func (h *ProductHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Ensure URL ID matches body ID
	if product.ID != id {
		http.Error(w, "ID mismatch between URL and body", http.StatusBadRequest)
		return
	}

	if err := h.storage.Update(product); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(product); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// Delete handles DELETE /products/{id}
func (h *ProductHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	if err := h.storage.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Product deleted successfully",
	}); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// List handles GET /products
func (h *ProductHandlers) List(w http.ResponseWriter, r *http.Request) {
	products := h.storage.List()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(products); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

func main() {
	// Initialize storage
	storage := NewProductStorage()

	// Initialize handlers
	handlers := NewProductHandlers(storage)

	// Register routes
	http.HandleFunc("POST /products", handlers.Create)
	http.HandleFunc("GET /products", handlers.List)
	http.HandleFunc("GET /products/{id}", handlers.Get)
	http.HandleFunc("PUT /products/{id}", handlers.Update)
	http.HandleFunc("DELETE /products/{id}", handlers.Delete)

	// Start server
	fmt.Println("Starting Fabrica simple REST API server...")
	fmt.Println("✓ Loaded Product handlers")
	fmt.Println("✓ Registered routes:")
	fmt.Println("  POST   /products")
	fmt.Println("  GET    /products")
	fmt.Println("  GET    /products/{id}")
	fmt.Println("  PUT    /products/{id}")
	fmt.Println("  DELETE /products/{id}")
	fmt.Println("")
	fmt.Println("Server listening on :8081")
	fmt.Println("")
	fmt.Println("Try it out:")
	fmt.Println("  curl -X POST http://localhost:8081/products \\")
	fmt.Println("    -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"id\":\"prod-1\",\"name\":\"Laptop\",\"price\":999.99,\"inStock\":true}'")
	fmt.Println("")

	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}

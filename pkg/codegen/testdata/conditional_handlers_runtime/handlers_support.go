// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"

	v1 "example.com/test/apis/v1"
)

type CreateNodeRequest struct {
	Metadata    v1.Metadata       `json:"metadata"`
	Spec        v1.NodeSpec       `json:"spec"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (r CreateNodeRequest) AsSpec() v1.NodeSpec {
	return r.Spec
}

type UpdateNodeRequest struct {
	Metadata    v1.Metadata       `json:"metadata"`
	Spec        v1.NodeSpec       `json:"spec"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (r UpdateNodeRequest) AsSpec() v1.NodeSpec {
	return r.Spec
}

type DeleteResponse struct {
	Message string `json:"message"`
	UID     string `json:"uid"`
}

func respondJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func respondError(w http.ResponseWriter, status int, err error) {
	respondJSON(w, status, map[string]string{"error": err.Error()})
}

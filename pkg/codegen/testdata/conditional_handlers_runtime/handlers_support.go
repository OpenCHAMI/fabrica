// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"

	v1 "example.com/test/apis/v1"
)

type CreateNodeRequest struct {
	Metadata        v1.Metadata       `json:"metadata" validate:"required"`
	Spec            v1.NodeSpec       `json:"spec" validate:"required"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	metadataPresent bool
	specPresent     bool
}

func (r *CreateNodeRequest) UnmarshalJSON(data []byte) error {
	type requestAlias CreateNodeRequest
	var decoded requestAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*r = CreateNodeRequest(decoded)
	metadata, hasMetadata := fields["metadata"]
	spec, hasSpec := fields["spec"]
	r.metadataPresent = hasMetadata && !bytes.Equal(bytes.TrimSpace(metadata), []byte("null"))
	r.specPresent = hasSpec && !bytes.Equal(bytes.TrimSpace(spec), []byte("null"))
	return nil
}

func (r *CreateNodeRequest) RequiredFieldPresent(name string) (bool, bool) {
	switch name {
	case "metadata":
		return r.metadataPresent || !reflect.DeepEqual(r.Metadata, v1.Metadata{}), true
	case "spec":
		return r.specPresent || !reflect.DeepEqual(r.Spec, v1.NodeSpec{}), true
	default:
		return false, false
	}
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

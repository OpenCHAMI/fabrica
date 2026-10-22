// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package resource_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/resource"
	"gopkg.in/yaml.v3"
)

func TestUIDMigrationSerialization(t *testing.T) {
	type spec struct {
		ReferenceUID string `json:"referenceUID" yaml:"referenceUID"`
	}
	type status struct {
		Phase string `json:"phase" yaml:"phase"`
	}
	type envelope struct {
		Metadata resource.Metadata `json:"metadata" yaml:"metadata"`
		Spec     spec              `json:"spec" yaml:"spec"`
		Status   status            `json:"status" yaml:"status"`
	}
	longUID, err := resource.GenerateUID(strings.Repeat("a", 60) + "_bc")
	if err != nil {
		t.Fatal(err)
	}
	if len(longUID) != 90 {
		t.Fatalf("maximum UID length = %d, want 90", len(longUID))
	}
	legacy := "disk1-ABCDEF12"
	for _, pair := range [][2]string{{legacy, longUID}, {longUID, legacy}} {
		want := envelope{
			Metadata: resource.Metadata{UID: pair[0], Name: "linked", Labels: map[string]string{"owner": "test"}, Annotations: map[string]string{"source": "migration"}},
			Spec:     spec{ReferenceUID: pair[1]}, Status: status{Phase: "Ready"},
		}
		for _, codec := range []struct {
			name      string
			marshal   func(interface{}) ([]byte, error)
			unmarshal func([]byte, interface{}) error
		}{{"JSON", json.Marshal, json.Unmarshal}, {"YAML", yaml.Marshal, yaml.Unmarshal}} {
			t.Run(codec.name+"/"+pair[0], func(t *testing.T) {
				data, err := codec.marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				var got envelope
				if err := codec.unmarshal(data, &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("serialization changed resource: got %+v, want %+v", got, want)
				}
			})
		}
	}
}

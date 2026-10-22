// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"encoding/json"
	"testing"
)

func TestUIDMigrationEventSerialization(t *testing.T) {
	previousConfig, previousBus := GetEventConfig(), GetGlobalEventBus()
	t.Cleanup(func() { SetEventConfig(previousConfig); SetGlobalEventBus(previousBus) })
	SetEventConfig(&EventConfig{Enabled: true, LifecycleEventsEnabled: true, ConditionEventsEnabled: true, EventTypePrefix: "io.test", ConditionEventPrefix: "io.test.condition", Source: "migration-api"})
	bus := &captureBus{}
	SetGlobalEventBus(bus)
	type payload struct {
		UID          string `json:"uid"`
		ReferenceUID string `json:"referenceUID"`
	}
	legacy, typeID := "node-ABCDEF12", "group_name_01h455vb4pex5vsknk084sn02q"
	for _, pair := range [][2]string{{legacy, typeID}, {typeID, legacy}} {
		for _, action := range []string{"created", "updated", "patched", "deleted", "condition"} {
			t.Run(action+"/"+pair[0], func(t *testing.T) {
				want := payload{UID: pair[0], ReferenceUID: pair[1]}
				var err error
				if action == "condition" {
					err = PublishConditionEvent(context.Background(), "Ready", "True", "Node", pair[0], want)
				} else {
					err = PublishResourceEvent(context.Background(), action, "Node", pair[0], want)
				}
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(bus.event)
				if err != nil {
					t.Fatal(err)
				}
				var decoded Event
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					t.Fatal(err)
				}
				if decoded.ResourceUID() != pair[0] || decoded.ResourceKind() != "Node" {
					t.Fatalf("event extensions changed: %s", encoded)
				}
				wantSource := "migration-api/resources/Node/" + pair[0]
				wantType := "io.test.node." + action
				if action == "condition" {
					wantSource += "/conditions/ready"
					wantType = "io.test.condition.ready"
				}
				if decoded.Source() != wantSource || decoded.Type() != wantType {
					t.Fatalf("event identity changed: %s", encoded)
				}
				var got payload
				if err := decoded.DataAs(&got); err != nil || got != want {
					t.Fatalf("event payload = %+v, %v; want %+v", got, err, want)
				}
			})
		}
	}
}

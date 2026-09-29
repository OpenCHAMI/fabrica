// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"testing"
)

type captureBus struct {
	event Event
}

func (*captureBus) Start() {}
func (b *captureBus) Publish(_ context.Context, event Event) error {
	b.event = event
	return nil
}
func (*captureBus) Subscribe(string, EventHandler) (SubscriptionID, error) { return "", nil }
func (*captureBus) Unsubscribe(SubscriptionID) error                       { return nil }
func (*captureBus) Close() error                                           { return nil }

func TestPublishRevisionEventIncludesSeriesAliasTransition(t *testing.T) {
	previousConfig := GetEventConfig()
	previousBus := GetGlobalEventBus()
	t.Cleanup(func() {
		SetEventConfig(previousConfig)
		SetGlobalEventBus(previousBus)
	})

	SetEventConfig(&EventConfig{Enabled: true, EventTypePrefix: "io.test", Source: "test-api"})
	bus := &captureBus{}
	SetGlobalEventBus(bus)
	payload := RevisionEventData{SeriesName: "production", RevisionUID: "rev_new", Alias: "default", PreviousUID: "rev_old", NewUID: "rev_new"}
	if err := PublishRevisionEvent(context.Background(), "alias-promoted", "BootConfig", payload); err != nil {
		t.Fatalf("PublishRevisionEvent() error = %v", err)
	}

	if got, want := bus.event.Type(), "io.test.bootconfig.revision.alias-promoted"; got != want {
		t.Fatalf("event type = %q, want %q", got, want)
	}
	var decoded RevisionEventData
	if err := bus.event.DataAs(&decoded); err != nil {
		t.Fatalf("DataAs() error = %v", err)
	}
	if decoded.SeriesName != payload.SeriesName || decoded.Alias != payload.Alias || decoded.PreviousUID != payload.PreviousUID || decoded.NewUID != payload.NewUID {
		t.Fatalf("event payload = %#v, want %#v", decoded, payload)
	}
}

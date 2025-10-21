// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alexlovelltroy/fabrica/pkg/events"
)

// TestEventConfiguration tests the basic event configuration functionality
func TestEventConfiguration(t *testing.T) {
	t.Run("CanSetConfiguration", func(t *testing.T) {
		config := &events.EventConfig{
			Enabled:                true,
			EventTypePrefix:        "test.resource",
			LifecycleEventsEnabled: true,
			ConditionEventsEnabled: false,
		}

		// This should not panic
		events.SetEventConfig(config)

		// Verify configuration was set
		retrievedConfig := events.GetEventConfig()
		assert.Equal(t, true, retrievedConfig.Enabled)
		assert.Equal(t, "test.resource", retrievedConfig.EventTypePrefix)
		assert.Equal(t, true, retrievedConfig.LifecycleEventsEnabled)
		assert.Equal(t, false, retrievedConfig.ConditionEventsEnabled)
	})

	t.Run("LifecycleEventsEnabledCheck", func(t *testing.T) {
		config := &events.EventConfig{
			Enabled:                true,
			LifecycleEventsEnabled: true,
		}
		events.SetEventConfig(config)

		assert.True(t, events.AreLifecycleEventsEnabled())

		config.LifecycleEventsEnabled = false
		events.SetEventConfig(config)

		assert.False(t, events.AreLifecycleEventsEnabled())
	})

	t.Run("ConditionEventsEnabledCheck", func(t *testing.T) {
		config := &events.EventConfig{
			Enabled:                true,
			ConditionEventsEnabled: true,
		}
		events.SetEventConfig(config)

		assert.True(t, events.AreConditionEventsEnabled())

		config.ConditionEventsEnabled = false
		events.SetEventConfig(config)

		assert.False(t, events.AreConditionEventsEnabled())
	})
}

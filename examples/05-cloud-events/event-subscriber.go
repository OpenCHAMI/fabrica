// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// event-subscriber.go - Example CloudEvents subscriber for monitoring sensor events
//
// This example demonstrates how to subscribe to and handle CloudEvents
// published by a Fabrica-generated API server. Run this alongside the
// sensor-monitor server to see events in real-time.
//
// Usage:
//
//	go run event-subscriber.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexlovelltroy/fabrica/pkg/events"
)

// Colors for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
)

// Event statistics
type EventStats struct {
	Total      int
	ByType     map[string]int
	ByResource map[string]int
	StartTime  time.Time
}

func NewEventStats() *EventStats {
	return &EventStats{
		ByType:     make(map[string]int),
		ByResource: make(map[string]int),
		StartTime:  time.Now(),
	}
}

func (s *EventStats) Record(eventType, resourceUID string) {
	s.Total++
	s.ByType[eventType]++
	s.ByResource[resourceUID]++
}

func (s *EventStats) PrintSummary() {
	duration := time.Since(s.StartTime)
	fmt.Printf("\n%s📊 Event Statistics (Runtime: %v)%s\n", ColorCyan, duration.Round(time.Second), ColorReset)
	fmt.Printf("   Total Events: %d\n", s.Total)

	if len(s.ByType) > 0 {
		fmt.Printf("   Events by Type:\n")
		for eventType, count := range s.ByType {
			fmt.Printf("     • %s: %d\n", eventType, count)
		}
	}

	if len(s.ByResource) > 0 {
		fmt.Printf("   Events by Resource:\n")
		for resource, count := range s.ByResource {
			fmt.Printf("     • %s: %d\n", resource, count)
		}
	}
}

func main() {
	fmt.Printf("%s🎧 CloudEvents Subscriber%s\n", ColorBlue, ColorReset)
	fmt.Printf("========================\n\n")

	// Create event bus - must match server configuration
	eventBus := events.NewInMemoryEventBus(1000, 1)
	eventBus.Start()
	defer eventBus.Close()

	stats := NewEventStats()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		fmt.Printf("\n%s🛑 Shutdown signal received...%s\n", ColorYellow, ColorReset)
		cancel()
	}()

	// Subscribe to all events with a comprehensive handler
	subscriptionID, err := eventBus.Subscribe("io.fabrica.**", func(ctx context.Context, event events.Event) error {
		handleEvent(event, stats)
		return nil
	})
	if err != nil {
		fmt.Printf("Failed to subscribe to events: %v\n", err)
		return
	}
	defer eventBus.Unsubscribe(subscriptionID)

	fmt.Printf("%s🔊 Listening for CloudEvents...%s\n", ColorGreen, ColorReset)
	fmt.Printf("   Event Bus: In-Memory\n")
	fmt.Printf("   Filter: All events\n")
	fmt.Printf("   Press Ctrl+C to stop and show statistics\n\n")

	// Print periodic statistics
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			stats.PrintSummary()
			fmt.Printf("\n%s👋 Event subscriber stopped%s\n", ColorGreen, ColorReset)
			return
		case <-ticker.C:
			if stats.Total > 0 {
				stats.PrintSummary()
			}
		}
	}
}

func handleEvent(event events.Event, stats *EventStats) {
	// Record statistics
	resourceUID := extractResourceUID(event)
	stats.Record(event.Type(), resourceUID)

	// Print event header with color coding
	color := getEventTypeColor(event.Type())
	fmt.Printf("%s🎯 Event Received%s\n", color, ColorReset)
	fmt.Printf("   Type: %s%s%s\n", color, event.Type(), ColorReset)
	fmt.Printf("   Source: %s\n", event.Source())
	fmt.Printf("   Subject: %s\n", event.Subject())
	fmt.Printf("   ID: %s\n", event.ID())
	fmt.Printf("   Time: %s\n", event.Time().Format(time.RFC3339))

	// Handle different event types
	switch {
	case isLifecycleEvent(event.Type()):
		handleLifecycleEvent(event)
	case isConditionEvent(event.Type()):
		handleConditionEvent(event)
	default:
		handleGenericEvent(event)
	}

	fmt.Printf("   %s────────────────────────────────────%s\n\n", ColorWhite, ColorReset)
}

func isLifecycleEvent(eventType string) bool {
	lifecycleTypes := []string{".created", ".updated", ".patched", ".deleted"}
	for _, suffix := range lifecycleTypes {
		if len(eventType) > len(suffix) && eventType[len(eventType)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

func isConditionEvent(eventType string) bool {
	return len(eventType) > 10 && eventType[:10] == "io.fabrica" &&
		(eventType[len(eventType)-6:] == ".ready" ||
			eventType[len(eventType)-8:] == ".healthy" ||
			eventType[len(eventType)-8:] == ".condition")
}

func handleLifecycleEvent(event events.Event) {
	var resourceData map[string]interface{}
	if err := event.DataAs(&resourceData); err != nil {
		fmt.Printf("   %sError parsing resource data: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	// Extract key information
	if metadata, ok := resourceData["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			fmt.Printf("   Resource: %s%s%s\n", ColorCyan, name, ColorReset)
		}
		if uid, ok := metadata["uid"].(string); ok {
			fmt.Printf("   UID: %s\n", uid)
		}
	}

	// Show spec information for created/updated events
	if spec, ok := resourceData["spec"].(map[string]interface{}); ok &&
		(event.Type()[len(event.Type())-8:] == ".created" || event.Type()[len(event.Type())-8:] == ".updated") {
		fmt.Printf("   Spec Changes:\n")
		for key, value := range spec {
			fmt.Printf("     • %s: %v\n", key, value)
		}
	}

	// Show status information for patched events
	if status, ok := resourceData["status"].(map[string]interface{}); ok &&
		event.Type()[len(event.Type())-8:] == ".patched" {
		fmt.Printf("   Status Updates:\n")
		for key, value := range status {
			if key == "conditions" {
				continue // Handle conditions separately
			}
			fmt.Printf("     • %s: %v\n", key, value)
		}
	}
}

func handleConditionEvent(event events.Event) {
	var conditionData struct {
		ResourceKind string `json:"resourceKind"`
		ResourceUID  string `json:"resourceUID"`
		Condition    struct {
			Type               string    `json:"type"`
			Status             string    `json:"status"`
			Reason             string    `json:"reason"`
			Message            string    `json:"message"`
			LastTransitionTime time.Time `json:"lastTransitionTime"`
		} `json:"condition"`
	}

	if err := event.DataAs(&conditionData); err != nil {
		fmt.Printf("   %sError parsing condition data: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	statusColor := ColorGreen
	if conditionData.Condition.Status == "False" {
		statusColor = ColorRed
	} else if conditionData.Condition.Status == "Unknown" {
		statusColor = ColorYellow
	}

	fmt.Printf("   Resource: %s%s (%s)%s\n", ColorCyan, conditionData.ResourceUID, conditionData.ResourceKind, ColorReset)
	fmt.Printf("   Condition: %s%s%s\n", ColorPurple, conditionData.Condition.Type, ColorReset)
	fmt.Printf("   Status: %s%s%s\n", statusColor, conditionData.Condition.Status, ColorReset)
	fmt.Printf("   Reason: %s\n", conditionData.Condition.Reason)
	fmt.Printf("   Message: %s\n", conditionData.Condition.Message)
	fmt.Printf("   Transition Time: %s\n", conditionData.Condition.LastTransitionTime.Format(time.RFC3339))
}

func handleGenericEvent(event events.Event) {
	// Pretty print the entire event data
	var data map[string]interface{}
	if err := event.DataAs(&data); err == nil {
		if jsonData, err := json.MarshalIndent(data, "   ", "  "); err == nil {
			fmt.Printf("   Data:\n%s\n", string(jsonData))
		}
	}
}

func extractResourceUID(event events.Event) string {
	// Try to extract from subject first (e.g., "sensors/temp-01")
	if subject := event.Subject(); subject != "" {
		parts := []rune(subject)
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == '/' {
				return string(parts[i+1:])
			}
		}
		return subject
	}

	// Try to extract from event data
	var data map[string]interface{}
	if err := event.DataAs(&data); err == nil {
		if metadata, ok := data["metadata"].(map[string]interface{}); ok {
			if uid, ok := metadata["uid"].(string); ok {
				return uid
			}
		}
		if resourceUID, ok := data["resourceUID"].(string); ok {
			return resourceUID
		}
	}

	return "unknown"
}

func getEventTypeColor(eventType string) string {
	switch {
	case eventType[len(eventType)-8:] == ".created":
		return ColorGreen
	case eventType[len(eventType)-8:] == ".updated":
		return ColorBlue
	case eventType[len(eventType)-8:] == ".patched":
		return ColorYellow
	case eventType[len(eventType)-8:] == ".deleted":
		return ColorRed
	case isConditionEvent(eventType):
		return ColorPurple
	default:
		return ColorWhite
	}
}

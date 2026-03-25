// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestAddTraceContextExtensions(t *testing.T) {
	event, err := NewEvent("com.test.created", "/tests", map[string]string{"ok": "true"})
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	traceID := trace.TraceID{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}
	spanID := trace.SpanID{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x10, 0x20}
	traceState, err := trace.ParseTraceState("vendor=value")
	if err != nil {
		t.Fatalf("failed to build trace state: %v", err)
	}

	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		TraceState: traceState,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)
	addTraceContextExtensions(ctx, event)

	traceparent, ok := event.Extensions()["traceparent"].(string)
	if !ok {
		t.Fatalf("expected traceparent extension")
	}

	expected := "00-101112131415161718191a1b1c1d1e1f-aabbccddeeff1020-01"
	if traceparent != expected {
		t.Fatalf("unexpected traceparent. got=%q want=%q", traceparent, expected)
	}

	if _, ok := event.Extensions()["tracestate"]; !ok {
		t.Fatalf("expected tracestate extension")
	}
}

func TestAddTraceContextExtensions_NoSpanContext(t *testing.T) {
	event, err := NewEvent("com.test.created", "/tests", map[string]string{"ok": "true"})
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	addTraceContextExtensions(context.Background(), event)

	if _, ok := event.Extensions()["traceparent"]; ok {
		t.Fatalf("did not expect traceparent without span context")
	}
	if _, ok := event.Extensions()["tracestate"]; ok {
		t.Fatalf("did not expect tracestate without span context")
	}
}
